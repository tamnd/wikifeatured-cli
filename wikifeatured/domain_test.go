package wikifeatured

import (
	"testing"

	"github.com/tamnd/any-cli/kit"
)

// These tests are offline: they exercise the domain's pure string functions
// and the host wiring, which need no network.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "wikifeatured" {
		t.Errorf("Scheme = %q, want wikifeatured", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "wikifeatured" {
		t.Errorf("Identity.Binary = %q, want wikifeatured", info.Identity.Binary)
	}
}

func TestClassifyErrors(t *testing.T) {
	_, _, err := Domain{}.Classify("anything")
	if err == nil {
		t.Error("Classify: want error, got nil")
	}
}

func TestLocateErrors(t *testing.T) {
	_, err := Domain{}.Locate("featured", "2026/06/14")
	if err == nil {
		t.Error("Locate: want error, got nil")
	}
}

func TestDomainRegistered(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}
	_ = h
}
