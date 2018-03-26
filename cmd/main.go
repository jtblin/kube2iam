package main

import (
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"

	"github.com/jtblin/kube2iam/iam"
	"github.com/jtblin/kube2iam/iptables"
	"github.com/jtblin/kube2iam/server"
	"github.com/jtblin/kube2iam/version"
)

// addFlags adds the command line flags.
func addFlags(s *server.Server, fs *pflag.FlagSet) {
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
	fs.BoolVar(&s.AutoDiscoverBaseArn, "auto-discover-base-arn", false, "Queries EC2 Metadata to determine the base ARN")
	fs.BoolVar(&s.AutoDiscoverDefaultRole, "auto-discover-default-role", false, "Queries EC2 Metadata to determine the default Iam Role and base ARN, cannot be used with --default-role, overwrites any previous setting for --base-role-arn")
	fs.StringVar(&s.HostInterface, "host-interface", "docker0", "Host interface for proxying AWS metadata")
	fs.BoolVar(&s.NamespaceRestriction, "namespace-restrictions", false, "Enable namespace restrictions")
	fs.StringVar(&s.NamespaceKey, "namespace-key", s.NamespaceKey, "Namespace annotation key used to retrieve the IAM roles allowed (value in annotation should be json array)")
	fs.StringVar(&s.HostIP, "host-ip", s.HostIP, "IP address of host")
	fs.StringVar(&s.NodeName, "node", s.NodeName, "Name of the node where kube2iam is running")
	fs.DurationVar(&s.BackoffMaxInterval, "backoff-max-interval", s.BackoffMaxInterval, "Max interval for backoff when querying for role.")
	fs.DurationVar(&s.BackoffMaxElapsedTime, "backoff-max-elapsed-time", s.BackoffMaxElapsedTime, "Max elapsed time for backoff when querying for role.")
	fs.StringVar(&s.LogFormat, "log-format", s.LogFormat, "Log format (text/json)")
	fs.StringVar(&s.LogLevel, "log-level", s.LogLevel, "Log level")
	fs.BoolVar(&s.Verbose, "verbose", false, "Verbose")
	fs.BoolVar(&s.Version, "version", false, "Print the version and exits")
}

func main() {
	s := server.NewServer()
	addFlags(s, pflag.CommandLine)
	pflag.Parse()

	logLevel, err := log.ParseLevel(s.LogLevel)
	if err != nil {
		log.Fatalf("%s", err)
	}

	if s.Verbose {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(logLevel)
	}

	if strings.ToLower(s.LogFormat) == "json" {
		log.SetFormatter(&log.JSONFormatter{})
	}

	if s.Version {
		version.PrintVersionAndExit()
	}

	if s.BaseRoleARN != "" {
		if !iam.IsValidBaseARN(s.BaseRoleARN) {
			log.Fatalf("Invalid --base-role-arn specified, expected: %s", iam.ARNRegexp.String())
		}
		if !strings.HasSuffix(s.BaseRoleARN, "/") {
			s.BaseRoleARN += "/"
		}
	}

	if s.AutoDiscoverBaseArn {
		if s.BaseRoleARN != "" {
			log.Fatal("--auto-discover-base-arn cannot be used if --base-role-arn is specified")
		}
		arn, err := iam.GetBaseArn()
		if err != nil {
			log.Fatalf("%s", err)
		}
		log.Infof("base ARN autodetected, %s", arn)
		s.BaseRoleARN = arn
	}

	if s.AutoDiscoverDefaultRole {
		if s.DefaultIAMRole != "" {
			log.Fatalf("You cannot use --default-role and --auto-discover-default-role at the same time")
		}
		arn, err := iam.GetBaseArn()
		if err != nil {
			log.Fatalf("%s", err)
		}
		s.BaseRoleARN = arn
		instanceIAMRole, err := iam.GetInstanceIAMRole()
		if err != nil {
			log.Fatalf("%s", err)
		}
		s.DefaultIAMRole = instanceIAMRole
		log.Infof("Using instance IAMRole %s%s as default", s.BaseRoleARN, s.DefaultIAMRole)
	}

	if s.AddIPTablesRule {
		if err := iptables.AddRule(s.AppPort, s.MetadataAddress, s.HostInterface, s.HostIP); err != nil {
			log.Fatalf("%s", err)
		}
	}

	if err := s.Run(s.APIServer, s.APIToken, s.NodeName, s.Insecure); err != nil {
		log.Fatalf("%s", err)
	}
}
