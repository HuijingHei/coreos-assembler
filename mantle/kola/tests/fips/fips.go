package fips

import (
	"github.com/coreos/coreos-assembler/mantle/kola"
	"github.com/coreos/coreos-assembler/mantle/kola/cluster"
	"github.com/coreos/coreos-assembler/mantle/kola/register"
	"github.com/coreos/coreos-assembler/mantle/platform/conf"
)

func init() {
	// Minimal test case to test FIPS enabling at first boot.
	// Also tests that using TLS works in FIPS mode by having Ignition
	// fetch a remote resource to make sure [1] doesn't happen again.
	// [1] https://issues.redhat.com/browse/OCPBUGS-65684
	register.RegisterTest(&register.Test{
		Run:         fipsEnableTest,
		ClusterSize: 1,
		Name:        `fips.enable`,
		Description: "Verify that fips enabled works.",
		Flags:       []register.Flag{},
		Tags:        []string{kola.NeedsInternetTag},
		Distros:     []string{"rhcos"},
		UserData: conf.Ignition(`{
			"ignition": {
				"config": {
					"replace": {
						"source": null,
						"verification": {}
					}
				},
				"security": {
					"tls": {}
				},
				"timeouts": {},
				"version": "3.0.0"
			},
			"passwd": {},
			"storage": {
				"files": [
					{
						"group": {
							"name": "root"
						},
						"overwrite": true,
						"path": "/etc/ignition-machine-config-encapsulated.json",
						"user": {
							"name": "root"
						},
						"contents": {
							"source": "data:,%7B%22metadata%22%3A%7B%22name%22%3A%22rendered-worker-1cc576110e0cf8396831ce4016f63900%22%2C%22selfLink%22%3A%22%2Fapis%2Fmachineconfiguration.openshift.io%2Fv1%2Fmachineconfigs%2Frendered-worker-1cc576110e0cf8396831ce4016f63900%22%2C%22uid%22%3A%2248871c03-899d-4332-a5f5-bef94e54b23f%22%2C%22resourceVersion%22%3A%224168%22%2C%22generation%22%3A1%2C%22creationTimestamp%22%3A%222019-11-04T15%3A54%3A08Z%22%2C%22annotations%22%3A%7B%22machineconfiguration.openshift.io%2Fgenerated-by-controller-version%22%3A%22bd846958bc95d049547164046a962054fca093df%22%7D%2C%22ownerReferences%22%3A%5B%7B%22apiVersion%22%3A%22machineconfiguration.openshift.io%2Fv1%22%2C%22kind%22%3A%22MachineConfigPool%22%2C%22name%22%3A%22worker%22%2C%22uid%22%3A%223d0dee9e-c9d6-4656-a4a9-81785b9ab01a%22%2C%22controller%22%3Atrue%2C%22blockOwnerDeletion%22%3Atrue%7D%5D%7D%2C%22spec%22%3A%7B%22osImageURL%22%3A%22registry.svc.ci.openshift.org%2Focp%2F4.3-2019-11-04-125204%40sha256%3A8a344c5b157bd01c3ca1abfcef0004fc39f5d69cac1cdaad0fd8dd332ad8e272%22%2C%22config%22%3A%7B%22ignition%22%3A%7B%22config%22%3A%7B%7D%2C%22security%22%3A%7B%22tls%22%3A%7B%7D%7D%2C%22timeouts%22%3A%7B%7D%2C%22version%22%3A%223.0.0%22%7D%2C%22networkd%22%3A%7B%7D%2C%22passwd%22%3A%7B%7D%2C%22storage%22%3A%7B%7D%2C%22systemd%22%3A%7B%7D%7D%2C%22kernelArguments%22%3A%5B%5D%2C%22fips%22%3Atrue%7D%7D",
							"verification": {}
						},
						"mode": 420
					},
					{
						"path": "/var/resource/https",
						"contents": {
							"source": "https://ignition-test-fixtures.s3.amazonaws.com/resources/anonymous"
						}
					}
				]
			}
		}`),
	})
	// We currently extract the FIPS config from an encapsulated Ignition
	// config provided by the Machine Config Operator. We test here that this
	// logic still works if custom partitions are present. This will no longer
	// be needed once Ignition understands FIPS directly.
	// This only works on QEMU as the device name (vda) is hardcoded.
	register.RegisterTest(&register.Test{
		Run:         fipsEnableTest,
		ClusterSize: 1,
		Name:        `fips.enable.partitions`,
		Description: "Verify that fips enabled works if custom partitions are present.",
		Flags:       []register.Flag{},
		Distros:     []string{"rhcos"},
		Platforms:   []string{"qemu"},
		UserData: conf.Ignition(`{
			"ignition": {
				"config": {
					"replace": {
						"source": null,
						"verification": {}
					}
				},
				"security": {
					"tls": {}
				},
				"timeouts": {},
				"version": "3.0.0"
			},
			"passwd": {},
			"storage": {
				"disks": [
					{
						"device": "/dev/vda",
						"partitions": [
							{
								"label": "CONTR",
								"sizeMiB": 0,
								"startMiB": 0
							}
						]
					}
				],
				"files": [
					{
						"group": {
							"name": "root"
						},
						"overwrite": true,
						"path": "/etc/ignition-machine-config-encapsulated.json",
						"user": {
							"name": "root"
						},
						"contents": {
							"source": "data:,%7B%22metadata%22%3A%7B%22name%22%3A%22rendered-worker-1cc576110e0cf8396831ce4016f63900%22%2C%22selfLink%22%3A%22%2Fapis%2Fmachineconfiguration.openshift.io%2Fv1%2Fmachineconfigs%2Frendered-worker-1cc576110e0cf8396831ce4016f63900%22%2C%22uid%22%3A%2248871c03-899d-4332-a5f5-bef94e54b23f%22%2C%22resourceVersion%22%3A%224168%22%2C%22generation%22%3A1%2C%22creationTimestamp%22%3A%222019-11-04T15%3A54%3A08Z%22%2C%22annotations%22%3A%7B%22machineconfiguration.openshift.io%2Fgenerated-by-controller-version%22%3A%22bd846958bc95d049547164046a962054fca093df%22%7D%2C%22ownerReferences%22%3A%5B%7B%22apiVersion%22%3A%22machineconfiguration.openshift.io%2Fv1%22%2C%22kind%22%3A%22MachineConfigPool%22%2C%22name%22%3A%22worker%22%2C%22uid%22%3A%223d0dee9e-c9d6-4656-a4a9-81785b9ab01a%22%2C%22controller%22%3Atrue%2C%22blockOwnerDeletion%22%3Atrue%7D%5D%7D%2C%22spec%22%3A%7B%22osImageURL%22%3A%22registry.svc.ci.openshift.org%2Focp%2F4.3-2019-11-04-125204%40sha256%3A8a344c5b157bd01c3ca1abfcef0004fc39f5d69cac1cdaad0fd8dd332ad8e272%22%2C%22config%22%3A%7B%22ignition%22%3A%7B%22config%22%3A%7B%7D%2C%22security%22%3A%7B%22tls%22%3A%7B%7D%7D%2C%22timeouts%22%3A%7B%7D%2C%22version%22%3A%223.0.0%22%7D%2C%22networkd%22%3A%7B%7D%2C%22passwd%22%3A%7B%7D%2C%22storage%22%3A%7B%7D%2C%22systemd%22%3A%7B%7D%7D%2C%22kernelArguments%22%3A%5B%5D%2C%22fips%22%3Atrue%7D%7D",
							"verification": {}
						},
						"mode": 420
					}
				],
				"filesystems": [
					{
						"device": "/dev/disk/by-partlabel/CONTR",
						"format": "xfs",
						"path": "/var/lib/containers",
						"wipeFilesystem": true
					}
				]
			},
			"systemd": {
				"units": [
					{
						"contents": "[Mount]\nWhat=/dev/disk/by-partlabel/CONTR\nWhere=/var/lib/containers\nType=xfs\nOptions=defaults\n[Install]\nWantedBy=local-fs.target",
						"enabled": true,
						"name": "var-lib-containers.mount"
					}
				]
			}
		}`),
	})
	// Test that using TLS works in FIPS mode by having Ignition fetch
	// a remote resource over HTTPS with FIPS compatible algorithms.
	// See https://issues.redhat.com/browse/COS-3487
	// Note that 34.136.148.229 (on GCP) is an HTTPS server powered by
	// nginx, delivering a small file exclusively over HTTPS using
	// FIPS-compliant algorithms.
	register.RegisterTest(&register.Test{
		Run:         fipsEnableTest,
		ClusterSize: 1,
		Name:        `fips.enable.https`,
		Description: "Verify that fips enabled works if fetching a remote resource over HTTPS with FIPS compatible algorithms.",
		Flags:       []register.Flag{},
		Tags:        []string{kola.NeedsInternetTag},
		Distros:     []string{"rhcos"},
		Platforms:   []string{"qemu"},
		UserData: conf.Ignition(`{
			"ignition": {
				"config": {
					"replace": {
						"source": null,
						"verification": {}
					}
				},
				"security": {
					"tls": {
						"certificateAuthorities": [
							{
								"compression": "gzip",
								"source": "data:;base64,H4sIAAAAAAAC/2SUyc67SBLE7zzF3NEIDMY2h/+hirUwYAqzGG4Ym93sUJinH33f9KW78/hLKZQpRcR/fwYqGrL/Iymuh1QkAU/5pZSFkGJUkgS6Uw4IgiBHfphDf1ec6UuCl7glnBA2aSZXtM8rRMaRce1iVKypDbBiUhADou/Ky4KTBg6+Agjx3brB/hf6nmrlgRbsLwl68cPg4gciepHaVhURaweCJacsZcuIC3/g/gu5H/jLKpBZLks0EskBxrIMXe0eQA+prvUjHj9sFql2Q6Vt3KQ/LyTon+dBiIGc55oDZEkCUSfluQaBOfiNtsJ1dDVZ3Y8UmwguZ3wCYxRuhfnkNNmxBPwsF9GrXqmPA21L6zbuc7CVn8XH7eHMFWKaYTcrxwQL1KOLuN5yDibGp5RVzZIJRvy8+OpSkio+uuOb6D3J6jcaX6+pbfJHaEVCvd0zwvInWaKYJWws1h9zyG+HXlU1Rr1r+Oltr63OVe5wjI5kXvDUzqIqrmO+qH09nbVeGxr2zpSQ0sni2BLzOIyGPdTZ0GV1XJzJZ3pnbO8P9f2cIZNlu/Xx9L+Aufpxrot6aDROsuJJCChQF2f4Pp+zOMZP83lT4toajH3sr6fnHShpvaNqJ0XUh8dPmbs9A1IbJ13iJMvzCfeequJdjWeeLlujylMzXcwsPAkKMpLytqKbu7sDU9nu5SCmxl2fVuKr9OmBw3yor1DAC6XXNF15hrSH73xwPWLV4NL3QlA9ir188q/0pI/z+65zZkTLimM7lhk9twv/yI5nPdI6ioF5ZFnO1XLyYRngh1/V01E33c9XDLKdyzRhChbaf9YtL3w4T5YwkgEGsDt0JJTBjYK5Hej4AkF2USCwJJgAouaRHLhsA7DOQECIlEfoSiIIsa8DohCZ/O5dCHJCwS5VUFf6ggde/xc7KmqOfU2TpNQ2mMRC5TvKUmk/XTiPzokO/uZY6idRf1kWAq29LTVnuygNHD7KnZKvP+Gtvr6KEykfHkGHIaTldoNWnU3R8crAM2XsheguE7Ju61HqrVPjJ/pMW5d7HY2emtIXYanZA/9q+sz9rNxc9qZEsuy9lCf7enitVN8zLK8+V/T9CpKUTNfKZD/f99cqR5kchLEjN63lp/myZffuI9d8V8ustt7riCv8XA+pZvYq2+EL/cJFGhwDus/y4fwyu/r65sak+AgXTmmbZb4nM7LWzdUmTpx59fE5BanRbhTig13rfe7qFS5kl++8uQFmkgN/9NAqJ/40QDxdZk2KgXs/mHz/8tJkwo6gMElSiw2VHeJWedl6aTf1zbKudvDQFJrZwgsX6HQbPObiLMYuCu4O6Rv2w1YBOBVtl3a0G3rLlXI8e9VaWbokE6vvDW2lhyVXSuGUI5HfrF7cjCkbTFEYWJugJWLe7Cl4h81tEY/z1kZUW/F8dJkekTKLGS1gWXJ2+vkuDFX3xm6bAsN4Sbsz6A9P3Phw55E5ztZZ79TpSLNAofbMVLubdwZtGOrgzx/qt6wVW/53gf8vAAD///rvDevdBQAA"
							}
						]
					}
				},
				"timeouts": {},
				"version": "3.4.0"
			},
			"passwd": {},
			"storage": {
				"files": [
					{
						"group": {
							"name": "root"
						},
						"overwrite": true,
						"path": "/etc/ignition-machine-config-encapsulated.json",
						"user": {
							"name": "root"
						},
						"contents": {
							"source": "data:,%7B%22metadata%22%3A%7B%22name%22%3A%22rendered-worker-1cc576110e0cf8396831ce4016f63900%22%2C%22selfLink%22%3A%22%2Fapis%2Fmachineconfiguration.openshift.io%2Fv1%2Fmachineconfigs%2Frendered-worker-1cc576110e0cf8396831ce4016f63900%22%2C%22uid%22%3A%2248871c03-899d-4332-a5f5-bef94e54b23f%22%2C%22resourceVersion%22%3A%224168%22%2C%22generation%22%3A1%2C%22creationTimestamp%22%3A%222019-11-04T15%3A54%3A08Z%22%2C%22annotations%22%3A%7B%22machineconfiguration.openshift.io%2Fgenerated-by-controller-version%22%3A%22bd846958bc95d049547164046a962054fca093df%22%7D%2C%22ownerReferences%22%3A%5B%7B%22apiVersion%22%3A%22machineconfiguration.openshift.io%2Fv1%22%2C%22kind%22%3A%22MachineConfigPool%22%2C%22name%22%3A%22worker%22%2C%22uid%22%3A%223d0dee9e-c9d6-4656-a4a9-81785b9ab01a%22%2C%22controller%22%3Atrue%2C%22blockOwnerDeletion%22%3Atrue%7D%5D%7D%2C%22spec%22%3A%7B%22osImageURL%22%3A%22registry.svc.ci.openshift.org%2Focp%2F4.3-2019-11-04-125204%40sha256%3A8a344c5b157bd01c3ca1abfcef0004fc39f5d69cac1cdaad0fd8dd332ad8e272%22%2C%22config%22%3A%7B%22ignition%22%3A%7B%22config%22%3A%7B%7D%2C%22security%22%3A%7B%22tls%22%3A%7B%7D%7D%2C%22timeouts%22%3A%7B%7D%2C%22version%22%3A%223.0.0%22%7D%2C%22networkd%22%3A%7B%7D%2C%22passwd%22%3A%7B%7D%2C%22storage%22%3A%7B%7D%2C%22systemd%22%3A%7B%7D%7D%2C%22kernelArguments%22%3A%5B%5D%2C%22fips%22%3Atrue%7D%7D",
							"verification": {}
						},
						"mode": 420
					},
					{
						"path": "/var/resource/https-fips",
						"contents": {
							"source": "https://34.136.148.229:8443/index.html"
						}
					}
				]
			}
		}`),
	})
}

// Test: Run basic FIPS test
func fipsEnableTest(c cluster.TestCluster) {
	m := c.Machines()[0]
	c.AssertCmdOutputContains(m, `cat /proc/sys/crypto/fips_enabled`, "1")
	c.AssertCmdOutputContains(m, `update-crypto-policies --show`, "FIPS")
}
