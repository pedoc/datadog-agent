// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2018 Datadog, Inc.

package config

// `aws-sdk-go` imports go-ini like this instead of `gopkg.in/ini.v1`, let's do
// the same to avoid checking in the dependency twice with different names.
import (
	"errors"
	"io/ioutil"

	"github.com/go-ini/ini"
	"gopkg.in/yaml.v2"

	"github.com/DataDog/datadog-agent/pkg/autodiscovery/integration"
)

// LegacyConfig is a simple key/value representation of the legacy agentConfig
// dictionary
type LegacyConfig map[string]string

type legacyConfigFormat struct {
	ADIdentifiers []string    `yaml:"ad_identifiers"`
	InitConfig    interface{} `yaml:"init_config"`
	Instances     []integration.RawMap
	DockerImages  []string `yaml:"docker_images"`
}

var (
	// Note: we'll only import a subset of these values.
	supportedValues = []string{
		"dd_url",
		"proxy_host",
		"proxy_port",
		"proxy_user",
		"proxy_password",
		"skip_ssl_validation",
		"api_key",
		"hostname",
		"tags",
		"forwarder_timeout",
		"default_integration_http_timeout",
		"collect_ec2_tags",
		"additional_checksd",
		"exclude_process_args",
		"histogram_aggregates",
		"histogram_percentiles",
		"service_discovery_backend",
		"sd_config_backend",
		"sd_backend_host",
		"sd_backend_port",
		"sd_backend_username",
		"sd_backend_password",
		"sd_template_dir",
		"consul_token",
		"use_dogstatsd",
		"dogstatsd_port",
		"statsd_metric_namespace",
		"log_level",
		"collector_log_file",
		"log_to_syslog",
		"log_to_event_viewer", // maybe deprecated, ignore for now
		"syslog_host",
		"syslog_port",
		"collect_instance_metadata",
		"listen_port",          // not for 6.0, ignore for now
		"non_local_traffic",    // not for 6.0, converted for the trace-agent
		"create_dd_check_tags", // not for 6.0, ignore for now
		"bind_host",
		"proxy_forbid_method_switch", // deprecated
		"collect_orchestrator_tags",  // deprecated
		"use_curl_http_client",       // deprecated
		"dogstatsd_target",           // deprecated
		"gce_updated_hostname",       // deprecated
		"process_agent_enabled",
		"disable_file_logging",
		"enable_gohai",
		// trace-agent specific
		"apm_enabled",
		"env",
		"receiver_port",
		"extra_sample_rate",
		"max_traces_per_second",
		"resource",
	}
)

// GetLegacyAgentConfig reads `datadog.conf` and returns a map that contains the same
// values as the agentConfig dictionary returned by `get_config()` in config.py
func GetLegacyAgentConfig(datadogConfPath string) (LegacyConfig, error) {
	config := make(map[string]string)
	iniFile, err := ini.Load(datadogConfPath)
	if err != nil {
		return config, err
	}

	// get the Main section
	main, err := iniFile.GetSection("Main")
	if err != nil {
		return config, err
	}

	// get the Trace sections
	// some of these aren't likely to exist
	traceConfig := iniFile.Section("trace.config")
	traceReceiver := iniFile.Section("trace.receiver")
	traceSampler := iniFile.Section("trace.sampler")
	traceIgnore := iniFile.Section("trace.ignore")

	// Grab the values needed to do a comparison of the Go vs Python algorithm.
	for _, supportedValue := range supportedValues {
		if value, err := main.GetKey(supportedValue); err == nil {
			config[supportedValue] = value.String()
		} else if value, err := traceConfig.GetKey(supportedValue); err == nil {
			config[supportedValue] = value.String()
		} else if value, err := traceReceiver.GetKey(supportedValue); err == nil {
			config[supportedValue] = value.String()
		} else if value, err := traceSampler.GetKey(supportedValue); err == nil {
			config[supportedValue] = value.String()
		} else if value, err := traceIgnore.GetKey(supportedValue); err == nil {
			config[supportedValue] = value.String()
		} else {
			// provide an empty default value so we don't need to check for
			// key existence when browsing the old configuration
			config[supportedValue] = ""
		}
	}

	// these are hardcoded in config.py
	config["graphite_listen_port"] = "None"
	config["watchdog"] = "True"
	config["use_forwarder"] = "False" // this doesn't come from the config file
	config["check_freq"] = "15"
	config["utf8_decoding"] = "False"
	config["ssl_certificate"] = "datadog-cert.pem"
	config["use_web_info_page"] = "True"

	// these values are postprocessed in config.py, manually overwrite them
	config["endpoints"] = "{}"
	config["version"] = "5.18.0"
	config["proxy_settings"] = "{'host': 'my-proxy.com', 'password': 'password', 'port': 3128, 'user': 'user'}"
	config["service_discovery"] = "True"

	return config, nil
}

func getLegacyIntegrationConfigFromFile(name, fpath string) (integration.Config, error) {
	cf := legacyConfigFormat{}
	config := integration.Config{Name: name}

	// Read file contents
	// FIXME: ReadFile reads the entire file, possible security implications
	yamlFile, err := ioutil.ReadFile(fpath)
	if err != nil {
		return config, err
	}

	// Parse configuration
	err = yaml.Unmarshal(yamlFile, &cf)
	if err != nil {
		return config, err
	}

	// If no valid instances were found this is not a valid configuration file
	if len(cf.Instances) < 1 {
		return config, errors.New("Configuration file contains no valid instances")
	}

	// at this point the Yaml was already parsed, no need to check the error
	if cf.InitConfig != nil {
		rawInitConfig, _ := yaml.Marshal(cf.InitConfig)
		config.InitConfig = rawInitConfig
	}

	// Go through instances and return corresponding []byte
	for _, instance := range cf.Instances {
		// at this point the Yaml was already parsed, no need to check the error
		rawConf, _ := yaml.Marshal(instance)
		config.Instances = append(config.Instances, rawConf)
	}

	// DockerImages entry was found: we ignore it if no ADIdentifiers has been found
	if len(cf.DockerImages) > 0 && len(cf.ADIdentifiers) == 0 {
		return config, errors.New("the 'docker_images' section is deprecated, please use 'ad_identifiers' instead")
	}

	return config, err
}
