package coretest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/pborman/uuid"

	"github.com/coreos/coreos-assembler/mantle/kola"
	"github.com/coreos/coreos-assembler/mantle/kola/cluster"
	"github.com/coreos/coreos-assembler/mantle/kola/register"
	"github.com/coreos/coreos-assembler/mantle/platform"
	"github.com/coreos/coreos-assembler/mantle/platform/machine/qemu"
)

const (
	DockerTimeout = time.Second * 60
	PortTimeout   = time.Second * 3
	uefi          = "uefi"
	uefiSecure    = "uefi-secure"
	bios          = "bios"
)

// RHCOS services we expect disabled/inactive
var offServices = []string{
	"dnsmasq.service",
	"nfs-blkmap.service",
	"nfs-idmapd.service",
	"nfs-mountd.service",
	"nfs-server.service",
	"nis-domainname.service",
	"rbdmap.service",
	"rdisc.service",
	"rpc-statd.service",
	"rpcbind.service",
	"rpcbind.socket",
	"tcsd.service",
}

var nativeFuncs = map[string]register.NativeFuncWrap{
	"PortSSH":        register.CreateNativeFuncWrap(TestPortSsh),
	"DbusPerms":      register.CreateNativeFuncWrap(TestDbusPerms),
	"ServicesActive": register.CreateNativeFuncWrap(TestServicesActive),
	"ReadOnly":       register.CreateNativeFuncWrap(TestReadOnlyFs),
	"Useradd":        register.CreateNativeFuncWrap(TestUseradd),
	"MachineID":      register.CreateNativeFuncWrap(TestMachineID),
	"RHCOSGrowpart":  register.CreateNativeFuncWrap(TestRHCOSGrowfs, []string{"fcos"}...),
	"FCOSGrowpart":   register.CreateNativeFuncWrap(TestFCOSGrowfs, []string{"rhcos"}...),
}

func init() {
	register.RegisterTest(&register.Test{
		Name:        "basic",
		Description: "Verify basic functionalities like SSH, systemd services, useradd, etc.",
		Run:         LocalTests,
		ClusterSize: 1,
		NativeFuncs: nativeFuncs,
	})
	register.RegisterTest(&register.Test{
		Name:          "basic.uefi",
		Description:   "Verify basic functionalities like SSH, systemd services, useradd, etc, with UEFI enabled",
		Run:           uefiWithBasicTests,
		Platforms:     []string{"qemu"},
		ClusterSize:   0,
		NativeFuncs:   nativeFuncs,
		Architectures: []string{"x86_64", "aarch64"},
	})
	register.RegisterTest(&register.Test{
		Name:          "basic.uefi-secure",
		Description:   "Verify basic functionalities like SSH, systemd services, useradd, etc, with UEFI Secure Boot enabled",
		Run:           uefiSecureWithBasicTests,
		Platforms:     []string{"qemu"},
		ClusterSize:   0,
		NativeFuncs:   nativeFuncs,
		Architectures: []string{"x86_64"},
	})
	register.RegisterTest(&register.Test{
		Name:        "basic.nvme",
		Description: "Verify basic functionalities like SSH, systemd services, useradd, etc, with nvme enabled",
		Run:         nvmeBasicTests,
		Platforms:   []string{"qemu"},
		ClusterSize: 0,
		NativeFuncs: nativeFuncs,
		// NVMe in theory is supported on all arches, but the way we test it seems to
		// only work on x86_64 and aarch64.
		Architectures: []string{"x86_64", "aarch64"},
	})
	register.RegisterTest(&register.Test{
		Name:        "rootfs.uuid",
		Description: "Verify that the root disk's GUID was set to a random one on first boot.",
		Run:         LocalTests,
		ClusterSize: 1,
		NativeFuncs: map[string]register.NativeFuncWrap{
			"RandomUUID": register.CreateNativeFuncWrap(TestFsRandomUUID),
		},
		// FIXME run on RHCOS once it has https://github.com/coreos/ignition-dracut/pull/93
		Distros: []string{"fcos"},
	})
	register.RegisterTest(&register.Test{
		Name:        "rhcos.services-disabled",
		Description: "Verify the specific services are disabled/inactive",
		Run:         LocalTests,
		ClusterSize: 1,
		NativeFuncs: map[string]register.NativeFuncWrap{
			"ServicesDisabled": register.CreateNativeFuncWrap(TestServicesDisabledRHCOS),
		},
		Distros: []string{"rhcos"},
	})
}

func uefiWithBasicTests(c cluster.TestCluster) {
	runBasicTests(c, uefi, false)
}

func uefiSecureWithBasicTests(c cluster.TestCluster) {
	runBasicTests(c, uefiSecure, false)
}

func nvmeBasicTests(c cluster.TestCluster) {
	runBasicTests(c, "", true)
}

func runBasicTests(c cluster.TestCluster, firmware string, nvme bool) {
	var err error
	var m platform.Machine

	options := platform.QemuMachineOptions{
		Firmware: firmware,
		Nvme:     nvme,
	}
	switch pc := c.Cluster.(type) {
	// These cases have to be separated because when put together to the same case statement
	// the golang compiler no longer checks that the individual types in the case have the
	// NewMachineWithQemuOptions function, but rather whether platform.Cluster
	// does which fails
	case *qemu.Cluster:
		m, err = pc.NewMachineWithQemuOptions(nil, options)
	default:
		panic("Unsupported cluster type")
	}
	if err != nil {
		c.Fatal(err)
	}

	// copy over kolet into the machine
	if err := kola.ScpKolet([]platform.Machine{m}); err != nil {
		c.Fatal(err)
	}
	LocalTests(c)
}

func TestPortSsh() error {
	//t.Parallel()
	err := CheckPort("tcp", "127.0.0.1:22", PortTimeout)
	if err != nil {
		return err
	}
	return nil
}

// This execs gdbus, because we need to change uses to test perms.
func TestDbusPerms() error {
	c := exec.Command(
		"sudo", "-u", "core",
		"gdbus", "call", "--system",
		"--dest", "org.freedesktop.systemd1",
		"--object-path", "/org/freedesktop/systemd1",
		"--method", "org.freedesktop.systemd1.Manager.RestartUnit",
		"ntpd.service", "replace",
	)
	out, err := c.CombinedOutput()

	if err != nil {
		if !strings.Contains(string(out), "org.freedesktop.DBus.Error.AccessDenied") &&
			!strings.Contains(string(out), "org.freedesktop.DBus.Error.InteractiveAuthorizationRequired") {
			return err
		}
	} else {
		return fmt.Errorf("We were able to call RestartUnit as a non-root user.")
	}

	c = exec.Command(
		"sudo", "-u", "core",
		"gdbus", "call", "--system",
		"--dest", "org.freedesktop.systemd1",
		"--object-path", "/org/freedesktop/systemd1/unit/ntpd_2eservice",
		"--method", "org.freedesktop.DBus.Properties.GetAll",
		"org.freedesktop.systemd1.Unit",
	)

	out, err = c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Err:%s\n Out:%v", err, out)
	}
	return nil
}

func TestServicesActive() error {
	return servicesActive([]string{
		"multi-user.target",
	})
}

func servicesActive(units []string) error {
	//t.Parallel()
	for _, unit := range units {
		c := exec.Command("systemctl", "is-active", unit)
		err := c.Run()
		if err != nil {
			return fmt.Errorf("Services Active: %v", err)
		}
	}
	return nil
}

func TestServicesDisabledRHCOS() error {
	err := servicesInactive(offServices)
	if err != nil {
		return err
	}

	err = servicesDisabled(offServices)
	if err != nil {
		return err
	}
	return nil
}

func servicesInactive(units []string) error {
	for _, unit := range units {
		c := exec.Command("systemctl", "is-active", unit)
		err := c.Run()
		if err == nil {
			return fmt.Errorf("Service Incorrectly Active: %q", unit)
		}
	}
	return nil
}

func servicesDisabled(units []string) error {
	for _, unit := range units {
		c := exec.Command("systemctl", "is-enabled", unit)
		out, err := c.Output()
		if err == nil {
			// "is-enabled" can return 0 in some cases when the output is not
			// explicitly "disabled".  In the case of the RHCOS services
			// that are checked, we expect some to report "static"
			outString := strings.TrimSuffix(string(out), "\n")
			if (outString != "disabled") && (outString != "static") {
				return fmt.Errorf("Service Incorrectly Enabled: %q", unit)
			}
		}
	}
	return nil
}

func TestReadOnlyFs() error {
	mountModes := make(map[string]bool)
	mounts, err := GetMountTable()
	if err != nil {
		return err
	}
	for _, m := range mounts {
		mountModes[m.MountPoint] = m.Options[0] == "ro"
	}
	if mp, ok := mountModes["/usr"]; ok {
		if mp {
			return nil
		} else {
			return fmt.Errorf("/usr is not mounted read-only.")
		}
	} else if mp, ok := mountModes["/"]; ok {
		if mp {
			return nil
		} else {
			return fmt.Errorf("/ is not mounted read-only.")
		}
	}
	return fmt.Errorf("could not find /usr or / mount points.")
}

// Test that the root disk's GUID was set to a random one on first boot.
func TestFsRandomUUID() error {
	c := exec.Command("sh", "-ec", "sudo blkid -o value -s PTUUID /dev/$(lsblk -no PKNAME $(findmnt -vno SOURCE /sysroot))")
	out, err := c.Output()
	if err != nil {
		return fmt.Errorf("findmnt: %v", err)
	}

	got, err := uuid.ParseBytes(bytes.TrimSpace(out))
	if err != nil {
		return fmt.Errorf("malformed GUID: %v", err)
	}

	defaultGUID := uuid.Parse("00000000-0000-4000-a000-000000000001")
	if uuid.Equal(defaultGUID, got) {
		return fmt.Errorf("unexpected default GUID found")
	}

	return nil
}

// Test "Add User Manually", from https://coreos.com/os/docs/latest/adding-users.html
func TestUseradd() error {
	u := "user1"
	c := exec.Command("sudo", "useradd", "-p", "*", "-U", "-m", u, "-G", "sudo")
	err := c.Run()
	if err != nil {
		return fmt.Errorf("useradd: %v", err)
	}

	// verify
	c = exec.Command("id", u)
	err = c.Run()
	if err != nil {
		return fmt.Errorf("id %s: %v", u, err)
	}

	return nil
}

// Test that /etc/machine-id isn't empty or COREOS_BLANK_MACHINE_ID
func TestMachineID() error {
	id := MachineID()
	if id == "" {
		return fmt.Errorf("machine-id is empty")
	} else if id == "COREOS_BLANK_MACHINE_ID" {
		return fmt.Errorf("machine-id is %s", id)
	}
	return nil
}

func testGrowfs(size int) error {
	// check that ignition-ostree-growfs.service was run and exited normally
	err := checkService("ignition-ostree-growfs.service")
	if err != nil {
		return err
	}
	err = checkFilesystemSize(size)
	if err != nil {
		return err
	}
	return nil
}

// TestRHCOSGrowfs tests whether ignition-ostree-growfs.service was run
// successfully and check that filesystem size has been grown to at least 15
// GB.
func TestRHCOSGrowfs() error {
	// check that filesystem size is >= 15 GB
	return testGrowfs(15 * 1024 * 1024 * 1024)
}

// TestFCOSGrowfs tests whether ignition-ostree-growfs.service was run successfully
// and check that filesystem size has been grown to at least 7 GB.
func TestFCOSGrowfs() error {
	// check that filesystem size is >= 7 GB
	return testGrowfs(7 * 1024 * 1024 * 1024)
}

func checkService(unit string) error {
	// Value of MESSAGE_ID= is the SD_MESSAGE_UNIT_STARTED macro's value from
	// https://github.com/systemd/systemd/blob/master/src/systemd/sd-messages.h
	// For oneshot type services that remain after exit, STARTED being "done"
	// should imply that the service ran and exited successfully.
	//
	// We add `--directory=/var/log/journal` here because we were seeing a race
	// condition starting on systemd 254 on s390x/ppc64le where we would get
	// two duplicate entries (one each from {/var,/run}log/journal/) returned
	// and it would break the json.Unmarshal below.
	c := exec.Command("journalctl", "-o", "json", "--directory=/var/log/journal",
		"MESSAGE_ID=39f53479d3a045ac8e11786248231fbf", "UNIT="+unit)
	out, err := c.Output()
	if err != nil {
		return fmt.Errorf("journalctl: %s", err)
	}
	if len(out) == 0 {
		return fmt.Errorf("%s did not start", unit)
	}
	var journalOutput map[string]string
	if err := json.Unmarshal(out, &journalOutput); err != nil {
		return fmt.Errorf("Error getting journalclt output for %s: %s. Out: %s", unit, err, out)
	}
	if journalOutput["JOB_RESULT"] != "done" {
		return fmt.Errorf("%s did not start successfully\n Journalctl output: %q", unit, out)
	}
	return nil
}

func checkFilesystemSize(size int) error {
	c := exec.Command("bash", "-c", "echo $(($(stat -f /sysroot --format '%S * %b')))")
	filesystemSizeStr, err := c.Output()
	if err != nil {
		return fmt.Errorf("Error getting filesystem size: %s", err)
	}
	filesystemSize, err := strconv.Atoi(strings.TrimSuffix(string(filesystemSizeStr), "\n"))
	if err != nil {
		return fmt.Errorf("Error converting filesystem size from string to int: %s", err)
	}
	if filesystemSize < size {
		return fmt.Errorf("Filesystem size is less than %d bytes, size in bytes: %d", size, filesystemSize)
	}
	return nil
}
