package sysctl

import (
	"io/ioutil"
	"path"
	"strings"

	"github.com/macula-io/macula-os/pkg/config"
)

func ConfigureSysctl(cfg *config.CloudConfig) error {
	for k, v := range cfg.Macula.Sysctls {
		elements := []string{"/proc", "sys"}
		elements = append(elements, strings.Split(k, ".")...)
		path := path.Join(elements...)
		if err := ioutil.WriteFile(path, []byte(v), 0644); err != nil {
			return err
		}
	}
	return nil
}
