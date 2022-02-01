//go:build linux && podman
// +build linux,podman

package podman

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/containers/podman/v3/pkg/bindings"
	"github.com/containers/podman/v3/pkg/bindings/containers"
	"github.com/containers/podman/v3/pkg/bindings/network"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/specgen"
	"github.com/dustin/go-humanize"
	"github.com/google/shlex"
	"github.com/opencontainers/runtime-spec/specs-go"
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var (
	errInvalidBind = errors.New("invalid bind mount provided")
)

type podmanWriterCloser struct {
	bytes.Buffer
}

func (*podmanWriterCloser) Close() error {
	return nil
}

func (*PodmanRuntime) connect(ctx context.Context) (context.Context, error) {
	return bindings.NewConnection(ctx, "unix://run/podman/podman.sock")
}

func (r *PodmanRuntime) createPodmanContainer(ctx context.Context, cfg *types.NodeConfig) (string, error) {
	sg, err := r.createContainerSpec(ctx, cfg)
	if err != nil {
		return "", fmt.Errorf("error while trying to create a container spec: %w", err)
	}
	res, err := containers.CreateWithSpec(ctx, &sg, &containers.CreateOptions{})
	log.Debugf("Created a container with ID %v, warnings %v and error %v", res.ID, res.Warnings, err)
	return res.ID, err
}

func (r *PodmanRuntime) createContainerSpec(ctx context.Context, cfg *types.NodeConfig) (specgen.SpecGenerator, error) {
	sg := specgen.SpecGenerator{}
	cmd, err := shlex.Split(cfg.Cmd)
	if err != nil {
		return sg, err
	}
	entrypoint, err := shlex.Split(cfg.Entrypoint)
	if err != nil {
		return sg, err
	}
	// Main container specs
	labels := cfg.Labels
	// encode a mgmt net name as an extra label
	labels["clab-net-mgmt"] = r.Mgmt.Network
	specBasicConfig := specgen.ContainerBasicConfig{
		Name:       cfg.LongName,
		Entrypoint: entrypoint,
		Command:    cmd,
		EnvHost:    false,
		HTTPProxy:  false,
		Env:        cfg.Env,
		Terminal:   false,
		Stdin:      false,
		Labels:     cfg.Labels,
		Hostname:   cfg.ShortName,
		Sysctl:     cfg.Sysctls,
		Remove:     false,
	}
	// Storage, image and mounts
	mounts, err := r.convertMounts(ctx, cfg.Binds)
	if err != nil {
		log.Errorf("Cannot convert mounts %v: %v", cfg.Binds, err)
		mounts = nil
	}
	specStorageConfig := specgen.ContainerStorageConfig{
		Image: cfg.Image,
		// Rootfs:            "",
		// ImageVolumeMode:   "",
		// VolumesFrom:       nil,
		// Init:              false,
		// InitPath:          "",
		Mounts: mounts,
		// Volumes:           nil,
		// OverlayVolumes:    nil,
		// ImageVolumes:      nil,
		// Devices:           nil,
		// DeviceCGroupRule:  nil,
		// IpcNS:             specgen.Namespace{},
		// ShmSize:           nil,
		// WorkDir:           "",
		// RootfsPropagation: "",
		// Secrets:           nil,
		// Volatile:          false,
	}
	// Security
	specSecurityConfig := specgen.ContainerSecurityConfig{
		Privileged: true,
		User:       cfg.User,
	}
	// Going with the defaults for cgroups
	specCgroupConfig := specgen.ContainerCgroupConfig{
		CgroupNS: specgen.Namespace{},
	}
	// Resource limits
	var (
		resLimits specs.LinuxResources
		lMem      specs.LinuxMemory
		lCPU      specs.LinuxCPU
	)
	// Memory limits
	if cfg.Memory != "" {
		mem, err := humanize.ParseBytes(cfg.Memory)
		mem64 := int64(mem)
		if err != nil {
			log.Warnf("Unable to parse memory limit %q for node %q", cfg.Memory, cfg.LongName)
		}
		lMem.Limit = &mem64
	}
	resLimits.Memory = &lMem
	// CPU resources limits
	if cfg.CPU != 0 {
		quota := int64(cfg.CPU * 100000)
		lCPU.Quota = &quota
		period := uint64(100000)
		lCPU.Period = &period
	}
	if cfg.CPUSet != "" {
		lCPU.Cpus = cfg.CPUSet
	}
	resLimits.CPU = &lCPU

	specResConfig := specgen.ContainerResourceConfig{
		ResourceLimits: &resLimits,
		// Rlimits:                 nil,
		// OOMScoreAdj:             nil,
		// WeightDevice:            nil,
		// ThrottleReadBpsDevice:   nil,
		// ThrottleWriteBpsDevice:  nil,
		// ThrottleReadIOPSDevice:  nil,
		// ThrottleWriteIOPSDevice: nil,
		// CgroupConf:              nil,
		// CPUPeriod:               0,
		// CPUQuota:                0,
	}
	// Defaults for health checks
	specHCheckConfig := specgen.ContainerHealthCheckConfig{}
	// Everything below is related to network spec of a container
	specNetConfig := specgen.ContainerNetworkConfig{}
	netns := cfg.NetworkMode
	switch netns {
	case "host":
		specNetConfig = specgen.ContainerNetworkConfig{
			NetNS: specgen.Namespace{NSMode: "host"},
			// UseImageResolvConf:  false,
			// DNSServers:          nil,
			// DNSSearch:           nil,
			// DNSOptions:          nil,
			UseImageHosts: false,
			HostAdd:       cfg.ExtraHosts,
			// NetworkOptions:      nil,
		}
	// Bridge will be used if none provided
	case "bridge", "":
		nets := []string{r.Mgmt.Network}
		mgmtv4Addr := net.ParseIP(cfg.MgmtIPv4Address)
		mgmtv6Addr := net.ParseIP(cfg.MgmtIPv6Address)
		mac, err := net.ParseMAC(cfg.MacAddress)
		if err != nil && cfg.MacAddress != "" {
			return sg, err
		}
		portmap, err := r.convertPortMap(ctx, cfg.PortBindings)
		if err != nil {
			return sg, err
		}
		expose, err := r.convertExpose(ctx, cfg.PortSet)
		if err != nil {
			return sg, err
		}
		specNetConfig = specgen.ContainerNetworkConfig{
			// Aliases:             nil,
			NetNS:               specgen.Namespace{NSMode: "bridge"},
			StaticIP:            &mgmtv4Addr,
			StaticIPv6:          &mgmtv6Addr,
			StaticMAC:           &mac,
			PortMappings:        portmap,
			PublishExposedPorts: false,
			Expose:              expose,
			CNINetworks:         nets,
			// UseImageResolvConf:  false,
			// DNSServers:          nil,
			// DNSSearch:           nil,
			// DNSOptions:          nil,
			UseImageHosts: false,
			HostAdd:       cfg.ExtraHosts,
			// NetworkOptions:      nil,
		}
	default:
		return sg, fmt.Errorf("network Mode %q is not currently supported with Podman", netns)
	}
	// Compile the final spec
	sg = specgen.SpecGenerator{
		ContainerBasicConfig:       specBasicConfig,
		ContainerStorageConfig:     specStorageConfig,
		ContainerSecurityConfig:    specSecurityConfig,
		ContainerCgroupConfig:      specCgroupConfig,
		ContainerNetworkConfig:     specNetConfig,
		ContainerResourceConfig:    specResConfig,
		ContainerHealthCheckConfig: specHCheckConfig,
	}
	return sg, nil
}

// convertMounts takes a list of filesystem mount binds in docker/clab format (src:dest:options)
// and converts it into an opencontainers spec format
func (*PodmanRuntime) convertMounts(_ context.Context, mounts []string) ([]specs.Mount, error) {
	if len(mounts) == 0 {
		return nil, nil
	}
	mntSpec := make([]specs.Mount, len(mounts))
	// Note: we don't do any input validation here
	for i, mnt := range mounts {
		mntSplit := strings.SplitN(mnt, ":", 3)

		if len(mntSplit) == 1 {
			return nil, fmt.Errorf("%w: %s", errInvalidBind, mnt)
		}

		mntSpec[i] = specs.Mount{
			Destination: mntSplit[1],
			Type:        "bind",
			Source:      mntSplit[0],
		}

		// when options are provided in the bind mount spec
		if len(mntSplit) == 3 {
			mntSpec[i].Options = strings.Split(mntSplit[2], ",")
		}
	}
	log.Debugf("convertMounts method received mounts %v and produced %+v as a result", mounts, mntSpec)
	return mntSpec, nil
}

// produceGenericContainerList takes a list of containers in a podman entities.ListContainer format
// and transforms it into a GenericContainer type
func (r *PodmanRuntime) produceGenericContainerList(ctx context.Context, cList []entities.ListContainer) ([]types.GenericContainer, error) {
	genericList := make([]types.GenericContainer, len(cList))
	for i, v := range cList {
		netSettings, err := r.extractMgmtIP(ctx, v.ID)
		if err != nil {
			return nil, err
		}
		genericList[i] = types.GenericContainer{
			Names:           v.Names,
			ID:              v.ID,
			ShortID:         v.ID[:12],
			Image:           v.Image,
			State:           v.State,
			Status:          v.Status,
			Labels:          v.Labels,
			Pid:             v.Pid,
			NetworkSettings: netSettings,
		}
	}
	log.Debugf("Method produceGenericContainerList returns %+v", genericList)
	return genericList, nil
}

func (*PodmanRuntime) extractMgmtIP(ctx context.Context, cID string) (types.GenericMgmtIPs, error) {
	// First get all the data from the inspect
	toReturn := types.GenericMgmtIPs{}
	inspectRes, err := containers.Inspect(ctx, cID, &containers.InspectOptions{})
	if err != nil {
		log.Warnf("Couldn't extract mgmt IPs for container %q, %v", cID, err)
	}
	// Extract the data only for a specific CNI. Network name is taken from a container's label
	netName, ok := inspectRes.Config.Labels["clab-net-mgmt"]
	if !ok || netName == "" {
		log.Warnf("Couldn't extract mgmt net data for container %q", cID)
		return toReturn, nil
	}
	mgmtData, ok := inspectRes.NetworkSettings.Networks[netName]
	if !ok || mgmtData == nil {
		log.Warnf("Couldn't extract mgmt IPs for container %q and net %q", cID, netName)
		return toReturn, nil
	}
	log.Debugf("extractMgmtIPs was called and we got a struct %T %+v", mgmtData, mgmtData)
	v4addr := mgmtData.IPAddress
	v4pLen := mgmtData.IPPrefixLen
	v6addr := mgmtData.GlobalIPv6Address
	v6pLen := mgmtData.GlobalIPv6PrefixLen

	toReturn = types.GenericMgmtIPs{
		IPv4addr: v4addr,
		IPv4pLen: v4pLen,
		IPv6addr: v6addr,
		IPv6pLen: v6pLen,
	}
	return toReturn, nil
}

func (r *PodmanRuntime) disableTXOffload(ctx context.Context) error {
	// TX checksum disabling will be done here since the mgmt bridge
	// may not exist in netlink before a container is attached to it
	netIns, err := network.Inspect(ctx, r.Mgmt.Network, &network.InspectOptions{})
	if err != nil {
		log.Warnf("failed to disable TX checksum offload; unable to retrieve the bridge name")
		return err
	}
	log.Debugf("Network Inspect result for the created net: type %T and values %+v", netIns, netIns)
	// Extract details for the bridge assuming that only 1 bridge was created for the network
	brName := netIns[0]["plugins"].([]interface{})[0].(map[string]interface{})["bridge"].(string)
	log.Debugf("Got a bridge name %q", brName)
	// Disable checksum calculation hw offload
	err = utils.EthtoolTXOff(brName)
	if err != nil {
		log.Warnf("failed to disable TX checksum offload for interface %q: %v", brName, err)
		return err
	}
	log.Debugf("Successully disabled Tx checksum offload for interface %q", brName)
	return nil
}

// netOpts is an accessory function that returns a network.CreateOptions struct
// filled with all parameters for CreateNet function
func (r *PodmanRuntime) netOpts(_ context.Context) (network.CreateOptions, error) {
	var (
		name       = r.Mgmt.Network
		driver     = "bridge"
		internal   = false
		ipv6       = false
		disableDNS = true
		options    = map[string]string{}
		labels     = map[string]string{"containerlab": ""}
		subnet     *net.IPNet
		err        error
	)
	if r.Mgmt.IPv4Subnet != "" {
		_, subnet, err = net.ParseCIDR(r.Mgmt.IPv4Subnet)
	}
	if err != nil {
		return network.CreateOptions{}, err
	}
	if r.Mgmt.MTU != "" {
		options["mtu"] = r.Mgmt.MTU
	}

	toReturn := network.CreateOptions{
		DisableDNS: &disableDNS,
		Driver:     &driver,
		Internal:   &internal,
		Labels:     labels,
		Subnet:     subnet,
		IPv6:       &ipv6,
		Options:    options,
		Name:       &name,
	}
	// add a custom gw address if specified
	if r.Mgmt.IPv4Gw != "" && r.Mgmt.IPv4Gw != "0.0.0.0" {
		toReturn.WithGateway(net.ParseIP(r.Mgmt.IPv4Gw))
	}
	// TODO: MTU?
	return toReturn, nil
}

func (*PodmanRuntime) buildFilterString(gFilters []*types.GenericFilter) map[string][]string {
	filters := map[string][]string{}
	for _, gF := range gFilters {
		filterType := gF.FilterType
		filterOp := gF.Operator
		filterValue := gF.Match
		if filterOp == "exists" {
			filterOp = "="
			filterValue = ""
		}
		if filterOp != "=" {
			log.Warnf("received a filter with unsupported match type: %+v", gF)
			continue
		}
		filterStr := gF.Field + filterOp + filterValue
		log.Debugf("produced a filterStr %q from inputs %+v", filterStr, gF)
		_, ok := filters[filterType]
		if !ok {
			filters[filterType] = []string{}
		}
		filters[filterType] = append(filters[filterType], filterStr)

	}
	log.Debugf("Method buildFilterString was called with inputs %+v\n and results %+v", gFilters, filters)
	return filters
}
