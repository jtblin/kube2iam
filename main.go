package main

import (
	"runtime"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/pflag"

	"github.com/jtblin/kube2iam/cmd"
	"github.com/jtblin/kube2iam/iptables"
	"github.com/jtblin/kube2iam/version"
)

const (
	defaultMaxInterval    = 1 * time.Second
	defaultMaxElapsedTime = 2 * time.Second
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	s := cmd.NewServer()
	addFlags(s, pflag.CommandLine)
	pflag.Parse()

	// default to info or above (probably the default anyways)
	log.SetLevel(log.InfoLevel)

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
	fs.BoolVar(&s.Debug, "debug", s.Debug, "Enable debug features")
	fs.StringVar(&s.DefaultIAMRole, "default-role", s.DefaultIAMRole, "Fallback role to use when annotation is not set")
	fs.StringVar(&s.IAMRoleKey, "iam-role-key", s.IAMRoleKey, "Pod annotation key used to retrieve the IAM role")
	fs.BoolVar(&s.Insecure, "insecure", false, "Kubernetes server should be accessed without verifying the TLS. Testing only")
	fs.StringVar(&s.MetadataAddress, "metadata-addr", s.MetadataAddress, "Address for the ec2 metadata")
	fs.BoolVar(&s.AddIPTablesRule, "iptables", false, "Add iptables rule (also requires --host-ip)")
	fs.StringVar(&s.HostInterface, "host-interface", "docker0", "Host interface for proxying AWS metadata")
	fs.BoolVar(&s.NamespaceRestriction, "namespace-restrictions", false, "Enable namespace restrictions")
	fs.StringVar(&s.NamespaceKey, "namespace-key", s.NamespaceKey, "Namespace annotation key used to retrieve the IAM roles allowed (value in annotation should be json array)")
	fs.StringVar(&s.HostIP, "host-ip", s.HostIP, "IP address of host")
	fs.DurationVar(&s.BackoffMaxInterval, "backoff-max-interval", defaultMaxInterval, "Max interval for backoff when querying for role.")
	fs.DurationVar(&s.BackoffMaxElapsedTime, "backoff-max-elapsed-time", defaultMaxElapsedTime, "Max elapsed time for backoff when querying for role.")
	fs.BoolVar(&s.Verbose, "verbose", false, "Verbose")
	fs.BoolVar(&s.Version, "version", false, "Print the version and exits")
}
