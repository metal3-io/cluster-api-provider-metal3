package e2e

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func createDirIfNotExist(dirPath string) {
	err := os.MkdirAll(dirPath, 0750)
	if err != nil && os.IsNotExist(err) {
		log.Fatal(err)
	}
}

func fetchContainerLogs(containerNames *[]string) {
	By("Create directories and storing container logs")
	for _, name := range *containerNames {
		containerRuntime := e2eConfig.GetVariable("CONTAINER_RUNTIME")
		logDir := filepath.Join(artifactFolder, containerRuntime, name)
		By(fmt.Sprintf("Create log directory for container %s at %s", name, logDir))
		createDirIfNotExist(logDir)
		By(fmt.Sprintf("Fetch logs for container %s", name))
		cmd := exec.Command("sudo", containerRuntime, "logs", name) // #nosec G204:gosec
		out, err := cmd.Output()
		if err != nil {
			writeErr := os.WriteFile(filepath.Join(logDir, "stderr.log"), []byte(err.Error()), 0444)
			Expect(writeErr).ToNot(HaveOccurred())
			log.Fatal(err)
		}
		writeErr := os.WriteFile(filepath.Join(logDir, "stdout.log"), out, 0444)
		Expect(writeErr).ToNot(HaveOccurred())
	}
}

