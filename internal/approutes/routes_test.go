package approutes

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveHelloBike(t *testing.T) {
	c, err := Open("")
	if err != nil {
		t.Fatal(err)
	}
	err = c.Add(App{
		Name:           "HelloBike",
		Scheme:         "hellobike://hellobike.com/",
		AndroidPackage: "com.jingyao.easybike",
		IOSBundleID:    "com.jingyao.easybike",
		Routes: []Route{
			{Name: "home", Path: "{scheme}"},
			{Name: "ride", Path: "{scheme}ride?bike={bikeId}"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.Resolve("HelloBike", "ride", "", map[string]string{"bikeId": "42"})
	if err != nil {
		t.Fatal(err)
	}
	if got.URI != "hellobike://hellobike.com/ride?bike=42" || got.AndroidPackage != "com.jingyao.easybike" || got.IOSBundleID != "com.jingyao.easybike" {
		t.Fatalf("%+v", got)
	}
	byPkg, err := c.Resolve("com.jingyao.easybike", "home", "", nil)
	if err != nil || byPkg.URI != "hellobike://hellobike.com/" {
		t.Fatalf("%+v %v", byPkg, err)
	}
	raw, err := c.Resolve("HelloBike", "", "hellobike://hellobike.com/scan", nil)
	if err != nil || raw.URI != "hellobike://hellobike.com/scan" || raw.AndroidPackage == "" {
		t.Fatalf("%+v %v", raw, err)
	}
	path := filepath.Join(t.TempDir(), "app-routes.yaml")
	c.path = path
	if err := c.save(); err != nil {
		t.Fatal(err)
	}
	again, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.List()) != 1 || again.List()[0].AndroidPackage != "com.jingyao.easybike" {
		t.Fatalf("%+v", again.List())
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
