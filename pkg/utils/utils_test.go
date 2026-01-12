package utils

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	linkPath   = "test/link"
	targetPath = "test/target"
)

func TestUtils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "utils test suite")
}

var _ = BeforeSuite(func() {
	log.SetFlags(0)
})

var _ = AfterEach(func() {
	_ = os.Remove(linkPath)
})

var _ = DescribeTable("test path resolution", // ResolvePath
	func(input string, output string, isSymlink bool, expectedErr error) {
		if isSymlink {
			targetPath, err := filepath.Abs(output)
			Expect(err).ToNot(HaveOccurred())

			err = os.Symlink(targetPath, input)
			Expect(err).ToNot(HaveOccurred())
		}

		err := ResolvePath(&input)
		Expect(input).To(Equal(output))

		if expectedErr != nil {
			Expect(err).To(Equal(expectedErr))
		}
	},
	Entry("path resolved without change", "/var/lib/kubelet/cpu_manager_state", "/var/lib/kubelet/cpu_manager_state", false, nil),
	Entry("path resolved with change", "/var/lib/../lib/kubelet/cpu_manager_state", "/var/lib/kubelet/cpu_manager_state", false, nil),
	Entry("empty path", "", "", false, fmt.Errorf("unable to resolve an empty path")),
	Entry("symbolic link", linkPath, getAbsPath(targetPath), true, nil),
)

var _ = DescribeTable("test flag resolution", // ResolveFlag
	func(flag string, path string, expectedResult string, expectedErr error) {
		err := ResolveFlag(flag, &path)
		Expect(path).To(Equal(expectedResult))

		if expectedErr != nil {
			Expect(err).To(Equal(expectedErr))
		}
	},
	Entry("flag resolved", "test_flag1", "/var/lib/kubelet/cpu_manager_state", "/var/lib/kubelet/cpu_manager_state", nil),
	Entry("empty path", "test_flag2", "", "", fmt.Errorf("test_flag2 - unable to resolve an empty path")),
)

var _ = DescribeTable("test IsSymLink", // IsSymLink
	func(fsys fs.FS, path string, expected bool) {
		Expect(IsSymLink(fsys, path)).To(Equal(expected))
	},
	Entry("with symlink", fstest.MapFS{"test_file": {Mode: fs.ModeSymlink}}, "test_file", true),
	Entry("without symlink", fstest.MapFS{"test_file": {Mode: fs.ModeDir}}, "test_file", false),
)

var _ = DescribeTable("test StringListFlag type", // StringListFlag
	func(input string, expectedSlice StringListFlag, expectedString string) {
		var list StringListFlag
		err := list.Set(input)

		Expect(err).ToNot(HaveOccurred())
		Expect(list).To(Equal(expectedSlice))
		Expect(list.String()).To(Equal(expectedString))
	},
	Entry("just one value", "sysfs", StringListFlag{"sysfs"}, "sysfs"),
	Entry("two values", "sysfs,netlink", StringListFlag{"sysfs", "netlink"}, "sysfs,netlink"),
	Entry("odd formatting", " sysfs ,   netlink ", StringListFlag{"sysfs", "netlink"}, "sysfs,netlink"),
)

func getAbsPath(fp string) string {
	absPath, err := filepath.Abs(fp)
	if err != nil {
		log.Printf("Failed to get absolute path, %v", err.Error())
	}
	return absPath
}
