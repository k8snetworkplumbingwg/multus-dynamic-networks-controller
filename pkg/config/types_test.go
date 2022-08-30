package config

import (
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/maiqueb/multus-dynamic-networks-controller/pkg/cri"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Dynamic network attachment configuration suite")
}

var _ = Describe("The dynamic network attachment configuration", func() {
	const allowAllPermissions = 0777

	var configurationDir string

	BeforeEach(func() {
		var err error
		configurationDir, err = os.MkdirTemp("", "multus-config")
		Expect(err).NotTo(HaveOccurred())

		Expect(os.MkdirAll(configurationDir, allowAllPermissions)).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(configurationDir)).To(Succeed())
	})

	When("a valid configuration file is provided", func() {
		const (
			criSocketPath    = "/path/to/socket"
			multusSocketPath = "/multus/socket/dir/socket.sock"
		)

		Context("default configuration values", func() {
			BeforeEach(func() {
				Expect(
					os.WriteFile(
						configurationFilePath(configurationDir),
						[]byte(configurationStringWithDefaultCRIType("", "")), allowAllPermissions),
				).To(Succeed())
			})

			It("the CRI type defaults to containerd runtime", func() {
				Expect(
					LoadConfig(configurationFilePath(configurationDir)),
				).To(
					WithTransform(configContainerRuntime, Equal(cri.Containerd)))
			})

			It("specifies a default for the multus socket directory", func() {
				Expect(
					LoadConfig(configurationFilePath(configurationDir)),
				).To(
					WithTransform(func(multusConfig *Multus) string {
						return multusConfig.MultusSocketPath
					}, Equal(defaultMultusRunDir)))
			})

			It("specifies the containerd socket as default", func() {
				Expect(
					LoadConfig(configurationFilePath(configurationDir)),
				).To(
					WithTransform(func(multusConfig *Multus) string {
						return multusConfig.CriSocketPath
					}, Equal(containerdSocketPath)))
			})
		})

		Context("overriding the configuration defaults", func() {
			BeforeEach(func() {
				Expect(
					os.WriteFile(
						configurationFilePath(configurationDir),
						[]byte(genericConfigString(criSocketPath, multusSocketPath, cri.Crio)), allowAllPermissions),
				).To(Succeed())
			})
			It("features the expected CRI socket path and multus socket directory", func() {
				Expect(
					LoadConfig(configurationFilePath(configurationDir)),
				).To(
					Equal(crioConfig(criSocketPath, multusSocketPath)))
			})
		})
	})

	It("fails when the config file is not present", func() {
		const aPath = "non-existent-path"
		_, err := LoadConfig(configurationFilePath(aPath))
		Expect(err).To(MatchError(nonExistentPathError(aPath)))
	})

	It("fails when the config file does not feature valid json", func() {
		Expect(
			os.WriteFile(
				configurationFilePath(configurationDir),
				[]byte("invalid-json"), allowAllPermissions),
		).To(Succeed())

		_, err := LoadConfig(configurationFilePath(configurationDir))
		Expect(err).To(MatchError(HavePrefix("failed to unmarshall the daemon configuration:")))
	})

	It("fails when the config file features an invalid runtime", func() {
		Expect(
			os.WriteFile(
				configurationFilePath(configurationDir),
				[]byte(genericConfigString("", "", "pony")), allowAllPermissions),
		).To(Succeed())

		_, err := LoadConfig(configurationFilePath(configurationDir))
		Expect(err).To(MatchError("invalid CRI type: pony. Allowed values are: containerd,crio"))
	})
})

func nonExistentPathError(configDir string) string {
	return fmt.Sprintf("failed to read the config file's contents: open %s/dummyconfig: no such file or directory", configDir)
}

func configContainerRuntime(multusConfig *Multus) cri.RuntimeType {
	return multusConfig.CriType
}

func crioConfig(criSocketPath string, multusSocketPath string) *Multus {
	return &Multus{
		CriSocketPath:    criSocketPath,
		CriType:          cri.Crio,
		MultusSocketPath: multusSocketPath,
	}
}

func genericConfigString(criSocketPath string, multusSocketPath string, runtime cri.RuntimeType) string {
	return fmt.Sprintf(`
{
    "criSocketPath": "%s",
    "multusSocketPath": "%s",
    "criType": "%s"
}`, criSocketPath, multusSocketPath, runtime)
}

func configurationStringWithDefaultCRIType(criSocketPath string, multusSocketPath string) string {
	return fmt.Sprintf(`
{
    "criSocketPath": "%s",
    "multusSocketPath": "%s"
}`, criSocketPath, multusSocketPath)
}

func configurationFilePath(configurationDir string) string {
	const filePath = "/dummyconfig"
	return configurationDir + filePath
}
