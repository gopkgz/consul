package consul

import (
	"fmt"
	"net"
	"strconv"

	consul "github.com/hashicorp/consul/api"
	"github.com/mkorenkov/microservice-prototype/nettools"
	"github.com/pkg/errors"
)

// Client essential consul API
type Client interface {
	Lookup(service string, tag string) (ipPort string, err error)
	Register(service string, address string, port int, healthcheck *consul.AgentServiceCheck) error
}

type client struct {
	consul *consul.Client
}

// NewConsulClient returns a Client interface for given consul address
func NewConsulClient(addr string) (Client, error) {
	config := consul.DefaultConfig()
	config.Address = addr
	c, err := consul.NewClient(config)
	if err != nil {
		return nil, err
	}
	return &client{consul: c}, nil
}

// NewgRPCHealthCheck creates very basic gRPC healthcheck for Consul
func NewgRPCHealthCheck(service string, address string, port int) *consul.AgentServiceCheck {
	return &consul.AgentServiceCheck{
		CheckID:       fmt.Sprintf("healthcheck-%s-%s:%d", service, address, port),
		Name:          fmt.Sprintf("grpc-%s", service),
		Interval:      "5s",
		Timeout:       "1s",
		TLSSkipVerify: true,
		GRPC:          net.JoinHostPort(address, strconv.Itoa(port)),
		GRPCUseTLS:    false,
	}
}

// Register a service with Consul
func (c *client) Register(service string, address string, port int, healthCheck *consul.AgentServiceCheck) error {
	reg := &consul.AgentServiceRegistration{
		ID:      fmt.Sprintf("%s-%s:%d", service, address, port),
		Name:    service,
		Address: address,
		Port:    port,
		Check:   healthCheck,
	}

	return c.consul.Agent().ServiceRegister(reg)
}

// Lookup find a healthy service in Consul
func (c *client) Lookup(service string, tag string) (string, error) {
	passingOnly := true
	addrs, _, err := c.consul.Health().Service(service, tag, passingOnly, nil)
	if err != nil {
		return "", errors.Wrap(err, "error getting service from consul")
	}
	if len(addrs) == 0 && err == nil {
		return "", errors.Errorf("service ( %s ) was not found", service)
	}
	// TODO: LB between services
	for _, serviceEntry := range addrs {
		if serviceEntry.Service.Address != "" && serviceEntry.Service.Port > 0 {
			return net.JoinHostPort(serviceEntry.Service.Address, strconv.Itoa(serviceEntry.Service.Port)), nil
		}
	}
	return "", errors.Errorf("Could not look up service %s host:port", service)
}

// RegistergRPCService fire and forget style registering of service in Consul
func RegistergRPCService(consulAddr string, myServiceID string, myServiceExternalAddr string) error {
	host, port, err := nettools.SplitHostPort(myServiceExternalAddr)
	if err != nil {
		return errors.Wrapf(err, "Cannot split host:port in %v", myServiceExternalAddr)
	}

	client, err := NewConsulClient(consulAddr)
	if err != nil {
		return errors.Wrap(err, "failed to connect to consul")
	}

	healthCheck := NewgRPCHealthCheck(consulAddr, host, port)

	err = client.Register("exception-service", host, port, healthCheck)
	if err != nil {
		return errors.Wrap(err, "failed to registed service")
	}
	return nil
}

// LookupService connects to consul on the given address and looks up for IP address of the service
func LookupService(consulAddr string, service string) (string, error) {
	client, err := NewConsulClient(consulAddr)
	if err != nil {
		return "", errors.Wrap(err, "failed to connect to consul")
	}
	addr, err := client.Lookup(service, "")
	if err != nil {
		return "", errors.Wrapf(err, "failed to lookup service %s", service)
	}
	return addr, nil
}
