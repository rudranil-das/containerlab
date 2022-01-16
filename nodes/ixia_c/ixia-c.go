// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ixia_c

import (
	"context"
	_ "embed"
	// "errors"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var (
	// defined env vars for the ixia-c
	ixiaCEnv = map[string]string{
		"OPT_LISTEN_PORT":					   "5555",
		"ARG_IFACE_LIST":					   "virtual@af_packet,eth1",
		"OPT_NO_HUGEPAGES":					   "Yes",
		// "SRC_ROOT":							   "/home/keysight/ixia-c/controller",
	}

	//go:embed ixia-c.cfg
	cfgTemplate string

	saveCmd = []string{"Cli", "-p", "15", "-c", "wr"}
)

func init() {
	nodes.Register(nodes.NodeKindIXIAC, func() nodes.Node {
		return new(ixia)
	})
}

type ixia struct {
	cfg     *types.NodeConfig
	runtime runtime.ContainerRuntime
}

func (s *ixia) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	s.cfg = cfg
	for _, o := range opts {
		o(s)
	}

	s.cfg.Env = utils.MergeStringMaps(ixiaCEnv, s.cfg.Env)

	// the node.Cmd should be aligned with the environment.
	var envSb strings.Builder
	envSb.WriteString("/sbin/init ")
	envSb.WriteString("./entrypoint.sh")
	// for k, v := range s.cfg.Env {
	// 	envSb.WriteString("systemd.setenv=" + k + "=" + v + " ")
	// }
	// envSb.WriteString("--accept-eula ")
	// envSb.WriteString("--debug ")
	// envSb.WriteString("--http-port ")
	// envSb.WriteString("5050 ")
	s.cfg.Cmd = envSb.String()
	s.cfg.MacAddress = utils.GenMac("00:1c:73")

	// mount config dir
	cfgPath := filepath.Join(s.cfg.LabDir, "flash")
	s.cfg.Binds = append(s.cfg.Binds, fmt.Sprintf("%s:/mnt/flash/", cfgPath))
	return nil
}

func (s *ixia) Config() *types.NodeConfig { return s.cfg }

func (s *ixia) PreDeploy(_, _, _ string) error {
	utils.CreateDirectory(s.cfg.LabDir, 0777)
	return createIXIAFiles(s.cfg)
}

func (s *ixia) Deploy(ctx context.Context) error {
	_, err := s.runtime.CreateContainer(ctx, s.cfg)
	return err
}

func (s *ixia) PostDeploy(ctx context.Context, _ map[string]nodes.Node) error {
	log.Infof("Running postdeploy actions for ixia-c '%s' node", s.cfg.ShortName)
	return ixiaPostDeploy(ctx, s.runtime, s.cfg)
}

func (*ixia) WithMgmtNet(*types.MgmtNet)               {}
func (s *ixia) WithRuntime(r runtime.ContainerRuntime) { s.runtime = r }
func (s *ixia) GetRuntime() runtime.ContainerRuntime   { return s.runtime }

func (s *ixia) SaveConfig(ctx context.Context) error {
	_, stderr, err := s.runtime.Exec(ctx, s.cfg.LongName, saveCmd)
	if err != nil {
		return fmt.Errorf("%s: failed to execute cmd: %v", s.cfg.ShortName, err)
	}

	if len(stderr) > 0 {
		return fmt.Errorf("%s errors: %s", s.cfg.ShortName, string(stderr))
	}

	confPath := s.cfg.LabDir + "/flash/startup-config"
	log.Infof("saved ixia-c configuration from %s node to %s\n", s.cfg.ShortName, confPath)

	return nil
}

func createIXIAFiles(node *types.NodeConfig) error {
	// generate config directory
	utils.CreateDirectory(path.Join(node.LabDir, "flash"), 0777)
	cfg := filepath.Join(node.LabDir, "flash", "startup-config")
	node.ResStartupConfig = cfg

	// use startup config file provided by a user
	if node.StartupConfig != "" {
		c, err := os.ReadFile(node.StartupConfig)
		if err != nil {
			return err
		}
		cfgTemplate = string(c)
	}

	err := node.GenerateConfig(node.ResStartupConfig, cfgTemplate)
	if err != nil {
		return err
	}

	// sysmac is a system mac that is +1 to Ma0 mac
	m, err := net.ParseMAC(node.MacAddress)
	if err != nil {
		return err
	}
	m[5] = m[5] + 1
	utils.CreateFile(path.Join(node.LabDir, "flash", "system_mac_address"), m.String())
	return nil
}

// ixiaPostDeploy runs postdeploy actions which are required for ixia nodes
func ixiaPostDeploy(_ context.Context, r runtime.ContainerRuntime, node *types.NodeConfig) error {
	// d, err := utils.SpawnCLIviaExec("arista_eos", node.LongName, r.GetName())
	// if err != nil {
	// 	return err
	// }

	var err error

	// defer d.Close()

	cfgs := []string{
		"interface management 0",
		"no ip address",
		"no ipv6 address",
	}

	// adding ipv4 address to configs
	if node.MgmtIPv4Address != "" {
		cfgs = append(cfgs,
			fmt.Sprintf("ip address %s/%d", node.MgmtIPv4Address, node.MgmtIPv4PrefixLength),
		)
	}

	// adding ipv6 address to configs
	if node.MgmtIPv6Address != "" {
		cfgs = append(cfgs,
			fmt.Sprintf("ipv6 address %s/%d", node.MgmtIPv6Address, node.MgmtIPv6PrefixLength),
		)
	}

	// add save to startup cmd
	cfgs = append(cfgs, "wr")

	// resp, err := d.SendConfigs(cfgs)
	if err != nil {
		return err
	} 

	return err
}

func (s *ixia) GetImages() map[string]string {
	return map[string]string{
		nodes.ImageKey: s.cfg.Image,
	}
}

func (s *ixia) Delete(ctx context.Context) error {
	return s.runtime.DeleteContainer(ctx, s.Config().LongName)
}