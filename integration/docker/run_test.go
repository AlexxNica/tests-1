// Copyright (c) 2017 Intel Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package docker

import (
	"fmt"
	"io/ioutil"
	"os"

	. "github.com/clearcontainers/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

// some machines support until 32 loop devices
var losetupMaxTries = 32

// number of loop devices to hotplug
var loopDevices = 10

func withWorkload(workload string, expectedExitCode int) TableEntry {
	return Entry(fmt.Sprintf("with '%v' as workload", workload), workload, expectedExitCode)
}

var _ = Describe("run", func() {
	var (
		args []string
		id   string
	)

	BeforeEach(func() {
		id = RandID(30)
		args = []string{"run", "--rm", "--name", id, Image, "sh", "-c"}
	})

	AfterEach(func() {
		Expect(ExistDockerContainer(id)).NotTo(BeTrue())
	})

	DescribeTable("container with docker",
		func(workload string, expectedExitCode int) {
			args = append(args, workload)

			command := NewCommand(Docker, args...)
			Expect(command).NotTo(BeNil())

			_, _, exitCode := command.Run()

			Expect(expectedExitCode).To(Equal(exitCode))
		},
		withWorkload("true", 0),
		withWorkload("false", 1),
		withWorkload("exit 0", 0),
		withWorkload("exit 1", 1),
		withWorkload("exit 15", 15),
		withWorkload("exit 123", 123),
	)
})

var _ = Describe("run", func() {
	var (
		args []string
		id   string
	)

	BeforeEach(func() {
		id = RandID(30)
		args = []string{"run", "--name", id}
	})

	AfterEach(func() {
		Expect(RemoveDockerContainer(id)).To(BeTrue())
		Expect(ExistDockerContainer(id)).NotTo(BeTrue())
	})

	DescribeTable("container with docker",
		func(options, expectedStatus string) {
			args = append(args, options, Image, "sh")

			command := NewCommand(Docker, args...)
			Expect(command).NotTo(BeNil())

			_, _, exitCode := command.Run()

			Expect(exitCode).To(BeZero())

			Expect(StatusDockerContainer(id)).To(Equal(expectedStatus))

			Expect(ExistDockerContainer(id)).To(BeTrue())
		},
		Entry("in background and interactive", "-di", "Up"),
		Entry("in background, interactive and with a tty", "-dit", "Up"),
	)
})

// creates a new disk file using 'dd' command, returns the path to disk file and
// its loop device representation
func createLoopDevice() (string, string, error) {
	f, err := ioutil.TempFile("", "dd")
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	// create disk file
	ddArgs := []string{"if=/dev/zero", fmt.Sprintf("of=%s", f.Name()), "count=1", "bs=5M"}
	ddCmd := NewCommand("dd", ddArgs...)
	if _, stderr, exitCode := ddCmd.Run(); exitCode != 0 {
		return "", "", fmt.Errorf("%s", stderr)
	}

	// partitioning disk file
	fdiskArgs := []string{"-c", fmt.Sprintf(`printf "g\nn\n\n\n\nw\n" | fdisk %s`, f.Name())}
	fdiskCmd := NewCommand("bash", fdiskArgs...)
	if _, stderr, exitCode := fdiskCmd.Run(); exitCode != 0 {
		return "", "", fmt.Errorf("%s", stderr)
	}

	// create loop device
	for i := 0; i < losetupMaxTries; i++ {
		loopPath := fmt.Sprintf("/dev/loop%d", i)
		losetupCmd := NewCommand("losetup", "-P", loopPath, f.Name())
		_, _, exitCode := losetupCmd.Run()
		if exitCode == 0 {
			return f.Name(), loopPath, nil
		}
	}

	return "", "", fmt.Errorf("unable to create loop device for disk %s", f.Name())
}

func deleteLoopDevice(loopFile string) error {
	partxCmd := NewCommand("losetup", "-d", loopFile)
	_, stderr, exitCode := partxCmd.Run()
	if exitCode != 0 {
		return fmt.Errorf("%s", stderr)
	}

	return nil
}

var _ = Describe("run", func() {
	var (
		err        error
		diskFiles  []string
		diskFile   string
		loopFiles  []string
		loopFile   string
		dockerArgs []string
		id         string
	)

	BeforeEach(func() {
		if os.Getuid() != 0 {
			Skip("only root user can create loop devices")
		}
		id = RandID(30)

		for i := 0; i < loopDevices; i++ {
			diskFile, loopFile, err = createLoopDevice()
			Expect(err).ToNot(HaveOccurred())

			diskFiles = append(diskFiles, diskFile)
			loopFiles = append(loopFiles, loopFile)
			dockerArgs = append(dockerArgs, "--device", loopFile)
		}

		dockerArgs = append(dockerArgs, "--rm", "--name", id, Image, "stat")

		for _, lf := range loopFiles {
			dockerArgs = append(dockerArgs, lf)
		}
	})

	AfterEach(func() {
		Expect(ExistDockerContainer(id)).NotTo(BeTrue())
		for _, lf := range loopFiles {
			err = deleteLoopDevice(lf)
			Expect(err).ToNot(HaveOccurred())
		}
		for _, df := range diskFiles {
			err = os.Remove(df)
			Expect(err).ToNot(HaveOccurred())
		}
	})

	Context("hot plug block devices", func() {
		It("should be attached", func() {
			_, _, exitCode := DockerRun(dockerArgs...)
			Expect(exitCode).To(BeZero())
		})
	})
})
