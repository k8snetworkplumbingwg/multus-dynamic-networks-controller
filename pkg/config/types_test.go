package config

import (
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

			It("specifies a default for the multus socket directory", func() {
				Expect(
					LoadConfig(configurationFilePath(configurationDir)),
				).To(
					WithTransform(func(multusConfig *Multus) string {
						return multusConfig.MultusSocketPath
					}, Equal(defaultMultusSocketPath)))
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
						[]byte(genericConfigString(criSocketPath, multusSocketPath)), allowAllPermissions),
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

})

func nonExistentPathError(configDir string) string {
	return fmt.Sprintf("failed to read the config file's contents: open %s/dummyconfig: no such file or directory", configDir)
}

func crioConfig(criSocketPath string, multusSocketPath string) *Multus {
	return &Multus{
		CriSocketPath:    criSocketPath,
		MultusSocketPath: multusSocketPath,
	}
}

func genericConfigString(criSocketPath string, multusSocketPath string) string {
	return fmt.Sprintf(`
{
    "criSocketPath": "%s",
    "multusSocketPath": "%s"
}`, criSocketPath, multusSocketPath)
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
