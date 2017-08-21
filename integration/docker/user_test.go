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
	"strings"

	. "github.com/clearcontainers/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

const (
	withAdditionalGroups    = true
	withoutAdditionalGroups = false
)

func asUser(user string, groups bool, fail bool) TableEntry {
	additionalGroups := []string{"cdrom", "games", "video", "audio"}
	groupsMsg := fmt.Sprintf(" with additional groups %v", additionalGroups)
	if !groups {
		groupsMsg = fmt.Sprintf(" without additional groups")
		additionalGroups = []string{}
	}

	return Entry(fmt.Sprintf("as '%s' user%s", user, groupsMsg),
		user, additionalGroups, fail)
}

var _ = Describe("users and groups", func() {
	var (
		id string
	)

	BeforeEach(func() {
		id = randomDockerName()
	})

	AfterEach(func() {
		Expect(ExistDockerContainer(id)).NotTo(BeTrue())
	})

	DescribeTable("running container",
		func(user string, additionalGroups []string, fail bool) {
			Skip("Issue https://github.com/clearcontainers/runtime/issues/386")

			cmd := []string{"--name", id, "--rm"}
			for _, ag := range additionalGroups {
				cmd = append(cmd, "--group-add", ag)
			}
			if user != "" {
				cmd = append(cmd, "-u", user)
			}
			cmd = append(cmd, "postgres", "id")

			stdout, stderr, exitCode := DockerRun(cmd...)
			if fail {
				Expect(exitCode).ToNot(Equal(0))
				Expect(stderr).NotTo(BeEmpty())
				// do not check stdout because container failed
				return
			}

			// check exit code and stderr
			Expect(exitCode).To(Equal(0))
			Expect(stderr).To(BeEmpty())

			var u, g string
			if user != "" {
				ug := strings.Split(user, ":")
				if len(ug) > 1 {
					u, g = ug[0], ug[1]
				} else {
					u, g = ug[0], ug[0]
				}
			}

			// default user and group is root
			if u == "" {
				u = "root"
			}
			if g == "" {
				g = "root"
			}

			fields := strings.Fields(stdout)

			// uid + gid + groups = 3
			Expect(fields).To(HaveLen(3))

			// check user (uid)
			Expect(fields[0]).To(ContainSubstring(fmt.Sprintf("(%s)", u)))

			// check group (gid)
			Expect(fields[1]).To(ContainSubstring(fmt.Sprintf("(%s)", g)))

			// check additional groups
			for _, ag := range additionalGroups {
				Expect(fields[2]).To(ContainSubstring(fmt.Sprintf("(%s)", ag)))
			}
		},
		asUser("", withAdditionalGroups, shouldNotFail),
		asUser("", withoutAdditionalGroups, shouldNotFail),
		asUser("root", withAdditionalGroups, shouldNotFail),
		asUser("root", withoutAdditionalGroups, shouldNotFail),
		asUser("postgres", withAdditionalGroups, shouldNotFail),
		asUser("postgres", withoutAdditionalGroups, shouldNotFail),
		asUser(":postgres", withAdditionalGroups, shouldNotFail),
		asUser(":postgres", withoutAdditionalGroups, shouldNotFail),
		asUser("postgres:postgres", withAdditionalGroups, shouldNotFail),
		asUser("postgres:postgres", withoutAdditionalGroups, shouldNotFail),
		asUser("root:postgres", withAdditionalGroups, shouldNotFail),
		asUser("root:postgres", withoutAdditionalGroups, shouldNotFail),
		asUser("nonexistentuser", withAdditionalGroups, shouldFail),
		asUser("nonexistentuser", withoutAdditionalGroups, shouldFail),
		asUser("nonexistentuser:postgres", withAdditionalGroups, shouldFail),
		asUser("nonexistentuser:postgres", withoutAdditionalGroups, shouldFail),
		asUser(":nonexistentuser", withAdditionalGroups, shouldFail),
		asUser(":nonexistentuser", withoutAdditionalGroups, shouldFail),
	)
})
