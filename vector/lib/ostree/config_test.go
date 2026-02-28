package ostree

import (
	"matrixos/vector/lib/config"
	"testing"
)

func TestConfigGettersErrors(t *testing.T) {
	cfg := &config.MockConfig{
		Items: map[string][]string{},
		Bools: map[string]bool{},
	}
	o, err := NewOstree(NewOstreeOptions{Config: cfg})
	if err != nil {
		t.Fatalf("NewOstree failed: %v", err)
	}

	if _, err := o.OsName(); err == nil {
		t.Error("OsName should fail with empty config")
	}
	if _, err := o.Arch(); err == nil {
		t.Error("Arch should fail with empty config")
	}
	if _, err := o.RepoDir(); err == nil {
		t.Error("RepoDir should fail with empty config")
	}
	if _, err := o.Sysroot(); err == nil {
		t.Error("Sysroot should fail with empty config")
	}
	if _, err := o.Remote(); err == nil {
		t.Error("Remote should fail with empty config")
	}
	if _, err := o.RemoteURL(); err == nil {
		t.Error("RemoteURL should fail with empty config")
	}
	if _, err := o.GpgPrivateKeyPath(); err == nil {
		t.Error("GpgPrivateKeyPath should fail with empty config")
	}
	if _, err := o.GpgPublicKeyPath(); err == nil {
		t.Error("GpgPublicKeyPath should fail with empty config")
	}
	if _, err := o.GpgOfficialPubKeyPath(); err == nil {
		t.Error("GpgOfficialPubKeyPath should fail with empty config")
	}
	if _, err := o.FullBranchSuffix(); err == nil {
		t.Error("FullBranchSuffix should fail with empty config")
	}
}
