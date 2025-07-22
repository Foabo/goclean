package main

import "testing"

func TestGetGOMODCACHE(t *testing.T) {
	gomodcache, err := GetGOMODCACHE()
	if err != nil {
		t.Fatalf("GetGOMODCACHE() failed: %v", err)
	}
	t.Logf("GOMODCACHE: %s", gomodcache)
}

func TestGetGOPATH(t *testing.T) {
	gopath, err := GetGOPATH()
	if err != nil {
		t.Fatalf("GetGOPATH() failed: %v", err)
	}
	t.Logf("GOPATH: %s", gopath)
}