package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCmdResetDeletesDB(t *testing.T) {
	dir := t.TempDir()
	st, err := openStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = st.SetSetting(setLinodeToken, "lt")
	_ = st.Close()
	enc := filepath.Join(dir, "hosts.sqlite.enc")
	if _, err := os.Stat(enc); err != nil {
		t.Fatal(err)
	}
	if err := cmdReset(Config{ConfigDir: dir}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(enc); !os.IsNotExist(err) {
		t.Fatalf("enc still there: %v", err)
	}
}


