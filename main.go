package main

import (
	"runtime"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/pflag"

	"github.com/jtblin/kube2iam/cmd"
	"github.com/jtblin/kube2iam/iptables"
	"github.com/jtblin/kube2iam/version"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	s := cmd.NewServer()
	addFlags(s, pflag.CommandLine)
	pflag.Parse()

	if s.Verbose {
		log.SetLevel(log.DebugLevel)
	}

	if s.Version {
		version.PrintVersionAndExit()
	}

	if s.AddIPTablesRule {
		if err := iptables.AddRule(s.AppPort, s.MetadataAddress, s.HostInterface, s.HostIP); err != nil {
			log.Fatal(err)
		}
	}

	if err := s.Run(s.APIServer, s.APIToken, s.Insecure); err != nil {
		log.Fatal(err)
	}
}

// addFlags adds the command line flags.
func addFlags(s *cmd.Server, fs *pflag.FlagSet) {
	fs.StringVar(&s.APIServer, "api-server", s.APIServer, "Endpoint for the api server")
	fs.StringVar(&s.APIToken, "api-token", s.APIToken, "Token to authenticate with the api server")
	fs.StringVar(&s.AppPort, "app-port", s.AppPort, "Http port")
	fs.StringVar(&s.BaseRoleARN, "base-role-arn", s.BaseRoleARN, "Base role ARN")
	fs.StringVar(&s.IAMRoleKey, "iam-role-key", s.IAMRoleKey, "Pod annotation key used to retrieve the IAM role")
	fs.BoolVar(&s.Insecure, "insecure", false, "Kubernetes server should be accessed without verifying the TLS. Testing only")
	fs.StringVar(&s.MetadataAddress, "metadata-addr", s.MetadataAddress, "Address for the ec2 metadata")
	fs.BoolVar(&s.AddIPTablesRule, "iptables", false, "Add iptables rule (also requires --host-ip)")
	fs.StringVar(&s.HostInterface, "host-interface", "docker0", "Host interface for proxying AWS metadata")
	fs.StringVar(&s.HostIP, "host-ip", s.HostIP, "IP address of host")
	fs.BoolVar(&s.Verbose, "verbose", false, "Verbose")
	fs.BoolVar(&s.Version, "version", false, "Print the version and exits")
}
