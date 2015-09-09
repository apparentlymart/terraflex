package terraflex

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hashicorp/terraform/plugin"
	"github.com/hashicorp/terraform/terraform"
	"github.com/mitchellh/osext"
)

type Plugins struct {
	providerNames    map[string]string
	provisionerNames map[string]string
}

func DiscoverPlugins() (*Plugins, error) {
	plugins := &Plugins{}

	// Look in the cwd.
	if err := discoverPluginsInDir(".", plugins); err != nil {
		return nil, err
	}

	// Next, look in the same directory as the executable. Any conflicts
	// will overwrite those found in our current directory.
	exePath, err := osext.Executable()
	if err != nil {
		log.Printf("[ERR] Error loading exe directory: %s", err)
	} else {
		if err := discoverPluginsInDir(filepath.Dir(exePath), plugins); err != nil {
			return nil, err
		}
	}

	return plugins, nil
}

func discoverPluginsInDir(path string, plugins *Plugins) error {
	var err error

	if !filepath.IsAbs(path) {
		path, err = filepath.Abs(path)
		if err != nil {
			return err
		}
	}

	err = discoverPluginsByPattern(
		filepath.Join(path, "terraform-provider-*"), &plugins.providerNames)
	if err != nil {
		return err
	}

	err = discoverPluginsByPattern(
		filepath.Join(path, "terraform-provisioner-*"), &plugins.provisionerNames)
	if err != nil {
		return err
	}

	return nil
}

func discoverPluginsByPattern(glob string, m *map[string]string) error {
	matches, err := filepath.Glob(glob)
	if err != nil {
		return err
	}

	if *m == nil {
		*m = make(map[string]string)
	}

	for _, match := range matches {
		file := filepath.Base(match)

		// If the filename has a ".", trim up to there
		if idx := strings.Index(file, "."); idx >= 0 {
			file = file[:idx]
		}

		// Look for foo-bar-baz. The plugin name is "baz"
		parts := strings.SplitN(file, "-", 3)
		if len(parts) != 3 {
			continue
		}

		(*m)[parts[2]] = match
	}

	return nil
}

func (p *Plugins) HasProvider(name string) bool {
	_, ok := p.providerNames[name]
	return ok
}

func (p *Plugins) HasProvisioner(name string) bool {
	_, ok := p.provisionerNames[name]
	return ok
}

func (p *Plugins) OpenProvider(name string) (terraform.ResourceProvider, error) {
	path, ok := p.providerNames[name]
	if !ok {
		return nil, fmt.Errorf("No provider named %#v", name)
	}

	var config plugin.ClientConfig
	config.Cmd = pluginCmd(path)
	config.Managed = true
	client := plugin.NewClient(&config)

	rpcClient, err := client.Client()
	if err != nil {
		return nil, err
	}

	return rpcClient.ResourceProvider()
}

func (p *Plugins) OpenProvisioner(name string) (terraform.ResourceProvisioner, error) {
	path, ok := p.provisionerNames[name]
	if !ok {
		return nil, fmt.Errorf("No provisioner named %#v", name)
	}

	var config plugin.ClientConfig
	config.Cmd = pluginCmd(path)
	config.Managed = true
	client := plugin.NewClient(&config)

	rpcClient, err := client.Client()
	if err != nil {
		return nil, err
	}

	return rpcClient.ResourceProvisioner()
}

func pluginCmd(path string) *exec.Cmd {
	cmdPath := ""

	// If the path doesn't contain a separator, look in the same
	// directory as the Terraform executable first.
	if !strings.ContainsRune(path, os.PathSeparator) {
		exePath, err := osext.Executable()
		if err == nil {
			temp := filepath.Join(
				filepath.Dir(exePath),
				filepath.Base(path))

			if _, err := os.Stat(temp); err == nil {
				cmdPath = temp
			}
		}

		// If we still haven't found the executable, look for it
		// in the PATH.
		if v, err := exec.LookPath(path); err == nil {
			cmdPath = v
		}
	}

	// If we still don't have a path, then just set it to the original
	// given path.
	if cmdPath == "" {
		cmdPath = path
	}

	// Build the command to execute the plugin
	return exec.Command(cmdPath)
}
