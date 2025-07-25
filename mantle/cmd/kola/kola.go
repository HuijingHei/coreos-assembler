// Copyright 2015 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/coreos/pkg/capnslog"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/coreos/coreos-assembler/mantle/cli"
	"github.com/coreos/coreos-assembler/mantle/fcos"
	"github.com/coreos/coreos-assembler/mantle/harness/reporters"
	"github.com/coreos/coreos-assembler/mantle/harness/testresult"
	"github.com/coreos/coreos-assembler/mantle/kola"
	"github.com/coreos/coreos-assembler/mantle/kola/register"
	"github.com/coreos/coreos-assembler/mantle/system"
	"github.com/coreos/coreos-assembler/mantle/util"
	cosa "github.com/coreos/coreos-assembler/pkg/builds"
	coreosarch "github.com/coreos/stream-metadata-go/arch"

	// register OS test suite
	_ "github.com/coreos/coreos-assembler/mantle/kola/registry"
)

var (
	plog = capnslog.NewPackageLogger("github.com/coreos/coreos-assembler/mantle", "kola")

	root = &cobra.Command{
		Use:   "kola [command]",
		Short: "The CoreOS Superdeep Borehole",
		// http://en.wikipedia.org/wiki/Kola_Superdeep_Borehole
	}

	cmdRun = &cobra.Command{
		Use:   "run [glob pattern...]",
		Short: "Run kola tests by category",
		Long: `Run all kola tests (default) or related groups.

If the glob pattern is exactly equal to the name of a single test, any
restrictions on the versions of Container Linux supported by that test
will be ignored.
`,
		RunE:         runRun,
		PreRunE:      preRun,
		SilenceUsage: true,
	}

	cmdRunUpgrade = &cobra.Command{
		Use:          "run-upgrade [glob pattern...]",
		Short:        "Run upgrade kola tests",
		Long:         `Run all upgrade kola tests (default) or related groups.`,
		RunE:         runRunUpgrade,
		PreRunE:      preRunUpgrade,
		SilenceUsage: true,
	}

	cmdList = &cobra.Command{
		Use:     "list",
		Short:   "List kola test names",
		PreRunE: preRun,
		RunE:    runList,

		SilenceUsage: true,
	}

	cmdHTTPServer = &cobra.Command{
		Use:   "http-server",
		Short: "Run a static webserver",
		Long: `Run a simple static webserver

This can be useful for e.g. serving locally built OSTree repos to qemu.
`,
		RunE: runHTTPServer,

		SilenceUsage: true,
	}

	cmdRerun = &cobra.Command{
		Use:     "rerun",
		Short:   "Rerun tests that failed in the last run",
		PreRunE: preRun,
		RunE:    runRerun,

		SilenceUsage: true,
	}

	cmdNcpu = &cobra.Command{
		Use:   "ncpu",
		Short: "Report the number of available CPUs for parallelism",
		RunE:  runNcpu,

		SilenceUsage: true,
	}

	listJSON           bool
	listPlatform       string
	listDistro         string
	httpPort           int
	findParentImage    bool
	qemuImageDir       string
	qemuImageDirIsTemp bool

	runExternals      []string
	runMultiply       int
	runRerunFlag      bool
	allowRerunSuccess string

	nonexclusiveWrapperMatch = regexp.MustCompile(`^non-exclusive-test-bucket-[0-9]$`)
)

func init() {
	root.AddCommand(cmdRun)
	cmdRun.Flags().StringArrayVarP(&runExternals, "exttest", "E", nil, "Externally defined tests (will be found in DIR/tests/kola)")
	cmdRun.Flags().IntVar(&runMultiply, "multiply", 0, "Run the provided tests N times (useful to find race conditions)")
	cmdRun.Flags().BoolVar(&runRerunFlag, "rerun", false, "re-run failed tests once")
	cmdRun.Flags().StringVar(&allowRerunSuccess, "allow-rerun-success", "", "Allow kola test run to be successful when tests with given 'tags=...[,...]' pass during re-run")

	root.AddCommand(cmdList)
	cmdList.Flags().StringArrayVarP(&runExternals, "exttest", "E", nil, "Externally defined tests in directory")
	cmdList.Flags().BoolVar(&listJSON, "json", false, "format output in JSON")
	cmdList.Flags().StringVarP(&listPlatform, "platform", "p", "all", "filter output by platform")
	cmdList.Flags().StringVarP(&listDistro, "distro", "b", "all", "filter output by distro")

	root.AddCommand(cmdHTTPServer)
	cmdHTTPServer.Flags().IntVarP(&httpPort, "port", "P", 8000, "Listen on provided port")

	root.AddCommand(cmdRunUpgrade)
	cmdRunUpgrade.Flags().BoolVar(&findParentImage, "find-parent-image", false, "automatically find parent image if not provided -- note on qemu, this will download the image")
	cmdRunUpgrade.Flags().StringVar(&qemuImageDir, "qemu-image-dir", "", "directory in which to cache QEMU images if --fetch-parent-image is enabled")
	cmdRunUpgrade.Flags().BoolVar(&runRerunFlag, "rerun", false, "re-run failed tests once")
	cmdRunUpgrade.Flags().StringVar(&allowRerunSuccess, "allow-rerun-success", "", "Allow kola test run to be successful when tests with given 'tags=...[,...]' pass during re-run")

	root.AddCommand(cmdRerun)

	root.AddCommand(cmdNcpu)
}

func main() {
	cli.Execute(root)
}

func preRun(cmd *cobra.Command, args []string) error {
	err := syncOptions()
	if err != nil {
		return err
	}

	// Reduce storage log level by default. We probably don't want to
	// reduce it in general, but in the "kola run" output the extra
	// messages are a distraction.
	if !plog.LevelAt(capnslog.INFO) {
		mantleLogger := capnslog.MustRepoLogger("github.com/coreos/coreos-assembler/mantle")
		mantleLogger.SetLogLevel(map[string]capnslog.LogLevel{
			"storage": capnslog.WARNING,
		})
	}

	return nil
}

func registerExternals() error {
	if err := kola.RegisterExternalTestsWithPrefix("/usr/lib/coreos-assembler", "ext"); err != nil {
		return err
	}
	for _, d := range runExternals {
		if d == "." {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			d = cwd
		}
		err := kola.RegisterExternalTests(d)
		if err != nil {
			return err
		}
	}
	return nil
}

func runRun(cmd *cobra.Command, args []string) error {
	var patterns []string
	if len(args) == 0 {
		patterns = []string{"*"} // run all tests by default
	} else {
		patterns = args
	}

	return kolaRunPatterns(patterns, runRerunFlag)
}

func runRerun(cmd *cobra.Command, args []string) error {
	var patterns []string
	data, err := reporters.DeserialiseReport(filepath.Join(kola.Options.CosaWorkdir, "tmp/kola/reports/report.json"))
	if err != nil {
		return err
	}
	for _, test := range data.Tests {
		if nonexclusiveWrapperMatch.MatchString(test.Name) {
			// When the test hasn't started yet, we get the subtests
			// of the test(nonExclusiveWrapper) for re-running
			for _, subtest := range test.Subtests {
				name, isRerunnable := kola.GetRerunnableTestName(subtest)
				if test.Result == testresult.Fail && isRerunnable {
					patterns = append(patterns, name)
				}
			}
		} else {
			name, isRerunnable := kola.GetRerunnableTestName(test.Name)
			if test.Result == testresult.Fail && isRerunnable {
				patterns = append(patterns, name)
			}
		}
	}
	return kolaRunPatterns(patterns, false)
}

// parseRerunSuccess converts rerun specification into a tags
func parseRerunSuccess() ([]string, error) {
	// In the future we may extend format to something like: <SELECTOR>[:<OPTIONS>]
	//   SELECTOR
	//     tags=...,...,...
	//     tests=...,...,...
	//   OPTIONS
	//     n=...
	//     allow-single=..
	tags := []string{}
	if len(allowRerunSuccess) == 0 {
		return tags, nil
	}
	if !strings.HasPrefix(allowRerunSuccess, "tags=") {
		return nil, fmt.Errorf("invalid rerun spec %s", allowRerunSuccess)
	}
	split := strings.TrimPrefix(allowRerunSuccess, "tags=")
	tags = append(tags, strings.Split(split, ",")...)
	return tags, nil
}

func kolaRunPatterns(patterns []string, rerun bool) error {
	var err error
	outputDir, err = kola.SetupOutputDir(outputDir, kolaPlatform)
	if err != nil {
		return err
	}

	if err := registerExternals(); err != nil {
		return err
	}

	rerunSuccessTags, err := parseRerunSuccess()
	if err != nil {
		return err
	}

	runErr := kola.RunTests(patterns, runMultiply, rerun, rerunSuccessTags, kolaPlatform, outputDir)

	// needs to be after RunTests() because harness empties the directory
	if err := writeProps(); err != nil {
		return err
	}

	return runErr
}

func writeProps() error {
	f, err := os.OpenFile(filepath.Join(outputDir, "properties.json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "    ")

	type AWS struct {
		Region       string `json:"region"`
		AMI          string `json:"ami"`
		InstanceType string `json:"type"`
	}
	type Azure struct {
		DiskURI           string `json:"diskUri"`
		Publisher         string `json:"publisher"`
		Offer             string `json:"offer"`
		Sku               string `json:"sku"`
		Version           string `json:"version"`
		Location          string `json:"location"`
		Size              string `json:"size"`
		AvailabilityZone  string `json:"availability_zone"`
		ManagedIdentityID string `json:"managed_identity_id"`
	}
	type DO struct {
		Region string `json:"region"`
		Size   string `json:"size"`
		Image  string `json:"image"`
	}
	type ESX struct {
		Server     string `json:"server"`
		BaseVMName string `json:"base_vm_name"`
	}
	type GCP struct {
		Image       string `json:"image"`
		MachineType string `json:"type"`
	}
	type OpenStack struct {
		Region string `json:"region"`
		Image  string `json:"image"`
		Flavor string `json:"flavor"`
	}
	type QEMU struct {
		Image     string `json:"image"`
		ImageSize string `json:"imageSize"`
		Swtpm     bool   `json:"swtpm"`
	}
	return enc.Encode(&struct {
		Cmdline     []string  `json:"cmdline"`
		Platform    string    `json:"platform"`
		Distro      string    `json:"distro"`
		Board       string    `json:"board"`
		OSContainer string    `json:"oscontainer"`
		AWS         AWS       `json:"aws"`
		Azure       Azure     `json:"azure"`
		DO          DO        `json:"do"`
		ESX         ESX       `json:"esx"`
		GCP         GCP       `json:"gcp"`
		OpenStack   OpenStack `json:"openstack"`
		QEMU        QEMU      `json:"qemu"`
	}{
		Cmdline:     os.Args,
		Platform:    kolaPlatform,
		Distro:      kola.Options.Distribution,
		OSContainer: kola.Options.OSContainer,
		AWS: AWS{
			Region:       kola.AWSOptions.Region,
			AMI:          kola.AWSOptions.AMI,
			InstanceType: kola.AWSOptions.InstanceType,
		},
		Azure: Azure{
			DiskURI:           kola.AzureOptions.DiskURI,
			Publisher:         kola.AzureOptions.Publisher,
			Offer:             kola.AzureOptions.Offer,
			Sku:               kola.AzureOptions.Sku,
			Version:           kola.AzureOptions.Version,
			Location:          kola.AzureOptions.Location,
			Size:              kola.AzureOptions.Size,
			AvailabilityZone:  kola.AzureOptions.AvailabilityZone,
			ManagedIdentityID: kola.AzureOptions.ManagedIdentityID,
		},
		DO: DO{
			Region: kola.DOOptions.Region,
			Size:   kola.DOOptions.Size,
			Image:  kola.DOOptions.Image,
		},
		ESX: ESX{
			Server:     kola.ESXOptions.Server,
			BaseVMName: kola.ESXOptions.BaseVMName,
		},
		GCP: GCP{
			Image:       kola.GCPOptions.Image,
			MachineType: kola.GCPOptions.MachineType,
		},
		OpenStack: OpenStack{
			Region: kola.OpenStackOptions.Region,
			Image:  kola.OpenStackOptions.Image,
			Flavor: kola.OpenStackOptions.Flavor,
		},
		QEMU: QEMU{
			Image:     kola.QEMUOptions.DiskImage,
			ImageSize: kola.QEMUOptions.DiskSize,
			Swtpm:     kola.QEMUOptions.Swtpm,
		},
	})
}

func runList(cmd *cobra.Command, args []string) error {
	if err := registerExternals(); err != nil {
		return err
	}
	var testlist []*item
	for name, test := range register.Tests {
		item := &item{
			name,
			test.Platforms,
			test.ExcludePlatforms,
			test.Architectures,
			test.ExcludeArchitectures,
			test.Distros,
			test.ExcludeDistros,
			test.Tags,
			test.Description}
		item.updateValues()
		testlist = append(testlist, item)
	}

	sort.Slice(testlist, func(i, j int) bool {
		return testlist[i].Name < testlist[j].Name
	})

	var newtestlist []*item
	for _, item := range testlist {
		platformFound := (listPlatform == "all")
		if listPlatform != "all" {
			for _, platform := range item.Platforms {
				if listPlatform == "all" || platform == "all" || platform == listPlatform {
					platformFound = true
					break
				}
			}
		}

		distroFound := (listDistro == "all")
		if listDistro != "all" {
			for _, distro := range item.Distros {
				if listDistro == "all" || distro == "all" || distro == listDistro {
					distroFound = true
					break
				}
			}
		}

		if platformFound && distroFound {
			newtestlist = append(newtestlist, item)
		}
	}

	if !listJSON {
		var w = tabwriter.NewWriter(os.Stdout, 0, 8, 0, '\t', 0)

		fmt.Fprintln(w, "Test Name\tPlatforms\tArchitectures\tDistributions\tTags")
		fmt.Fprintln(w, "\t")
		for _, item := range newtestlist {
			fmt.Fprintf(w, "%v\n", item)
		}
		w.Flush()
	} else {
		out, err := json.MarshalIndent(newtestlist, "", "\t")
		if err != nil {
			return errors.Wrapf(err, "marshalling test list")
		}
		fmt.Println(string(out))
	}

	return nil
}

type item struct {
	Name                 string
	Platforms            []string
	ExcludePlatforms     []string `json:"-"`
	Architectures        []string
	ExcludeArchitectures []string `json:"-"`
	Distros              []string
	ExcludeDistros       []string `json:"-"`
	Tags                 []string
	Description          string
}

func (i *item) updateValues() {
	buildItems := func(include, exclude, all []string) []string {
		if len(include) == 0 && len(exclude) == 0 {
			if listJSON {
				return all
			}
			return []string{"all"}
		}
		var retItems []string
		if len(exclude) > 0 {
			excludeMap := map[string]struct{}{}
			for _, item := range exclude {
				excludeMap[item] = struct{}{}
			}
			if len(include) == 0 {
				retItems = all
			} else {
				retItems = include
			}
			items := []string{}
			for _, item := range retItems {
				if _, ok := excludeMap[item]; !ok {
					items = append(items, item)
				}
			}
			retItems = items
		} else {
			retItems = include
		}
		return retItems
	}
	i.Platforms = buildItems(i.Platforms, i.ExcludePlatforms, kolaPlatforms)
	i.Architectures = buildItems(i.Architectures, nil, kolaArchitectures)
	i.Distros = buildItems(i.Distros, i.ExcludeDistros, kolaDistros)
}

func (i item) String() string {
	return fmt.Sprintf("%v\t%v\t%v\t%v\t%v", i.Name, i.Platforms, i.Architectures, i.Distros, i.Tags)
}

func runHTTPServer(cmd *cobra.Command, args []string) error {
	directory := "."

	http.Handle("/", http.FileServer(http.Dir(directory)))

	fmt.Fprintf(os.Stdout, "Serving HTTP on port: %d\n", httpPort)
	return http.ListenAndServe(fmt.Sprintf(":%d", httpPort), nil)
}

func preRunUpgrade(cmd *cobra.Command, args []string) error {
	// note we pass `false` here for useCosa because we want to customize the
	// *starting* image for upgrade tests
	err := syncOptionsImpl(false)
	if err != nil {
		return err
	}

	// Unlike `kola run`, we *require* a cosa build. We check this after
	// syncOptionsImpl so that it may be auto-filled in based on either an
	// explicit `--build` or $PWD or `--workdir`.
	if kola.Options.CosaBuildId == "" {
		return errors.New("Error: missing required argument --build")
	}

	if findParentImage {
		err = syncFindParentImageOptions()
		if err != nil {
			runUpgradeCleanup()
			return err
		}
	}

	return nil
}

func runUpgradeCleanup() {
	if qemuImageDir != "" && qemuImageDirIsTemp {
		os.RemoveAll(qemuImageDir)
	}
}

// syncFindParentImageOptions handles --find-parent-image automagic.
func syncFindParentImageOptions() error {
	var err error

	var parentBaseURL string
	skipSignature := false
	switch kola.Options.Distribution {
	case "fcos":
		// We're taking liberal shortcuts here... the cleaner way to do this is
		// parse commitmeta.json for `fedora-coreos.stream`
		if kola.CosaBuild.Meta.BuildRef == "" {
			return errors.New("no ref in build metadata")
		}
		stream := filepath.Base(kola.CosaBuild.Meta.BuildRef)
		parentBaseURL, err = getParentFcosBuildBase(stream)
		if err != nil {
			return err
		}
	case "rhcos":
		// Hardcoded for now based on https://github.com/openshift/installer/blob/release-4.6/data/data/rhcos.json
		tag := "rhcos-4.6"
		release := "46.82.202011260640-0"
		switch coreosarch.CurrentRpmArch() {
		case "s390x":
			tag += "-" + coreosarch.CurrentRpmArch()
			release = "46.82.202011261339-0"
		case "ppc64le":
			tag += "-" + coreosarch.CurrentRpmArch()
			release = "46.82.202011260639-0"
		}
		parentBaseURL = fmt.Sprintf("https://rhcos.mirror.openshift.com/art/storage/releases/%s/%s/%s/", tag, release, coreosarch.CurrentRpmArch())
		// sigh...someday we'll get the stuff signed by ART or maybe https://github.com/openshift/enhancements/pull/201 will just happen
		skipSignature = true
	default:
		return fmt.Errorf("--find-parent-image not yet supported for distro %s", kola.Options.Distribution)
	}

	var parentCosaBuild *cosa.Build
	parentCosaBuild, err = cosa.FetchAndParseBuild(parentBaseURL + "meta.json")
	if err != nil {
		return err
	}

	// Here we handle the --fetch-parent-image --> platform-specific options
	// based on its cosa build metadata
	switch kolaPlatform {
	case "qemu":
		if qemuImageDir == "" {
			if qemuImageDir, err = os.MkdirTemp("/var/tmp", "kola-run-upgrade"); err != nil {
				return err
			}
			qemuImageDirIsTemp = true
		}
		if parentCosaBuild.BuildArtifacts.Qemu == nil {
			return fmt.Errorf("No QEMU in parent meta.json")
		}
		qcowURL := parentBaseURL + parentCosaBuild.BuildArtifacts.Qemu.Path
		qcowLocal := filepath.Join(qemuImageDir, parentCosaBuild.BuildArtifacts.Qemu.Path)
		decompressedQcowLocal, err := util.DownloadImageAndDecompress(qcowURL, qcowLocal, skipSignature)
		if err != nil {
			return err
		}
		kola.QEMUOptions.DiskImage = decompressedQcowLocal
	case "aws":
		kola.AWSOptions.AMI, err = parentCosaBuild.FindAMI(kola.AWSOptions.Region)
		if err != nil {
			return err
		}
	case "gcp":
		kola.GCPOptions.Image, err = parentCosaBuild.FindGCPImage()
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("--find-parent-image not yet supported for platform %s", kolaPlatform)
	}

	return nil
}

// Returns the URL to a parent build that can be used as a base for upgrade
// testing.
func getParentFcosBuildBase(stream string) (string, error) {
	// For FCOS, we can be clever and automagically fetch the metadata for the
	// parent release, which should be the latest release on that stream.

	// We're taking liberal shortcuts here... the cleaner way to do this is to
	// fetch the stream metadata, then fetch the release metadata

	var parentVersion string
	if kola.CosaBuild.Meta.FedoraCoreOsParentVersion != "" {
		parentVersion = kola.CosaBuild.Meta.FedoraCoreOsParentVersion
	} else {
		// ok, we're probably operating on a local dev build or in the
		// bump-lockfile job since the pipeline always injects the
		// parent; just instead fetch the release index for that stream
		// and get the last build id from there
		index, err := fcos.FetchAndParseCanonicalReleaseIndex(stream)
		if err != nil {
			return "", err
		}
		// as we build for multi-architectures now,
		// inorder to allow failures to be non-fatal and
		// enable them to promote to non-prod streams
		// when any specific previous release is unavailable for that arch.
		// in that case, releases are searched in reverse order
		// and the most recent release which has the secondary arch is considered.
		for _, release := range index.Releases {
			for _, commit := range release.Commits {
				if commit.Architecture == (kola.Options.CosaBuildArch) {
					parentVersion = release.Version
					break
				}
			}
		}
		if parentVersion == "" {
			msg := fmt.Sprintf("no parent version in build metadata, and no build on stream %s", stream)
			if stream == "testing-devel" {
				return "", errors.New(msg)
			}
			plog.Infof("%s; falling back to testing-devel", msg)
			return getParentFcosBuildBase("testing-devel")
		}
	}

	return fcos.GetCosaBuildURL(stream, parentVersion, kola.Options.CosaBuildArch), nil
}

func runRunUpgrade(cmd *cobra.Command, args []string) error {
	defer runUpgradeCleanup()

	outputDir, err := kola.SetupOutputDir(outputDir, kolaPlatform)
	if err != nil {
		return err
	}

	var patterns []string
	if len(args) == 0 {
		patterns = []string{"*"} // run all tests by default
	} else {
		patterns = args
	}

	runErr := kola.RunUpgradeTests(patterns, runRerunFlag, kolaPlatform, outputDir)

	// needs to be after RunTests() because harness empties the directory
	if err := writeProps(); err != nil {
		return err
	}

	return runErr
}

func runNcpu(cmd *cobra.Command, args []string) error {
	count, err := system.GetProcessors()
	if err != nil {
		return err
	}
	fmt.Println(count)
	return nil
}
