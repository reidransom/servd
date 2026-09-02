//go:build !windows

package commands

import (
	"os"
	"strconv"
	"testing"
)

func TestValidateBindIdentityAcceptsActiveCallerGroups(t *testing.T) {
	uid := uint32(os.Getuid())
	gid := uint32(os.Getgid())
	activeGroups, err := os.Getgroups()
	if err != nil {
		t.Fatal(err)
	}
	groups := make([]uint32, len(activeGroups))
	for index, group := range activeGroups {
		groups[index] = uint32(group)
	}
	t.Setenv("SUDO_UID", strconv.FormatUint(uint64(uid), 10))
	t.Setenv("SUDO_GID", strconv.FormatUint(uint64(gid), 10))

	if err := validateBindIdentity(uid, gid, groups); err != nil {
		t.Fatalf("validate active caller groups: %v", err)
	}
}

func TestValidateSupplementaryGroupsUsesActiveGroupVector(t *testing.T) {
	if err := validateSupplementaryGroups(1000, []uint32{992}, []int{992, 998}); err != nil {
		t.Fatalf("validate active supplementary group: %v", err)
	}

	err := validateSupplementaryGroups(1000, []uint32{992}, []int{998})
	if err == nil {
		t.Fatal("validate inactive supplementary group succeeded")
	}
}
