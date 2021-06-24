// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package vr_sros

import (
	"context"
	"fmt"
	"path"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

const (
	vrsrosDefaultType = "sr-1"
)

func init() {
	nodes.Register(nodes.NodeKindVrSROS, func() nodes.Node {
		return new(vrSROS)
	})
}

type vrSROS struct {
	cfg  *types.NodeConfig
	mgmt *types.MgmtNet
}

func (s *vrSROS) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	s.cfg = cfg
	for _, o := range opts {
		o(s)
	}
	if s.cfg.Config == "" {
		s.cfg.Config = nodes.DefaultConfigTemplates[s.cfg.Kind]
	}
	// vr-sros type sets the vrnetlab/sros variant (https://github.com/hellt/vrnetlab/sros)
	if s.cfg.NodeType == "" {
		s.cfg.NodeType = vrsrosDefaultType
	}
	// env vars are used to set launch.py arguments in vrnetlab container
	defEnv := map[string]string{
		"CONNECTION_MODE":    nodes.VrDefConnMode,
		"DOCKER_NET_V4_ADDR": s.mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": s.mgmt.IPv6Subnet,
	}
	s.cfg.Env = utils.MergeStringMaps(defEnv, s.cfg.Env)

	// mount tftpboot dir
	s.cfg.Binds = append(s.cfg.Binds, fmt.Sprint(path.Join(s.cfg.LabDir, "tftpboot"), ":/tftpboot"))
	if s.cfg.Env["CONNECTION_MODE"] == "macvtap" {
		// mount dev dir to enable macvtap
		s.cfg.Binds = append(s.cfg.Binds, "/dev:/dev")
	}

	s.cfg.Cmd = fmt.Sprintf("--trace --connection-mode %s --hostname %s --variant \"%s\"", s.cfg.Env["CONNECTION_MODE"],
		s.cfg.ShortName,
		s.cfg.NodeType,
	)
	return nil
}

func (s *vrSROS) Config() *types.NodeConfig { return s.cfg }

func (s *vrSROS) PreDeploy(configName, labCADir, labCARoot string) error {
	utils.CreateDirectory(s.cfg.LabDir, 0777)
	return createVrSROSFiles(s.cfg)
}

func (s *vrSROS) Deploy(ctx context.Context, r runtime.ContainerRuntime) error {
	return r.CreateContainer(ctx, s.cfg)
}

func (s *vrSROS) PostDeploy(ctx context.Context, r runtime.ContainerRuntime, ns map[string]nodes.Node) error {
	return nil
}

func (s *vrSROS) WithMgmtNet(mgmt *types.MgmtNet) {
	s.mgmt = mgmt
}

//

func createVrSROSFiles(node *types.NodeConfig) error {
	// create config directory that will be bind mounted to vrnetlab container at / path
	utils.CreateDirectory(path.Join(node.LabDir, "tftpboot"), 0777)

	if node.License != "" {
		// copy license file to node specific lab directory
		src := node.License
		dst := path.Join(node.LabDir, "/tftpboot/license.txt")
		if err := utils.CopyFile(src, dst); err != nil {
			return fmt.Errorf("file copy [src %s -> dst %s] failed %v", src, dst, err)
		}
		log.Debugf("CopyFile src %s -> dst %s succeeded", src, dst)

		cfg := path.Join(node.LabDir, "tftpboot", "config.txt")
		if node.Config != "" {
			err := node.GenerateConfig(cfg, nodes.DefaultConfigTemplates[node.Kind])
			if err != nil {
				log.Errorf("node=%s, failed to generate config: %v", node.ShortName, err)
			}
		} else {
			log.Debugf("Config file exists for node %s", node.ShortName)
		}
	}
	return nil
}
