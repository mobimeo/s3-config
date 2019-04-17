package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"github.com/Songmu/prompter"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/google/subcommands"
	"github.com/sergi/go-diff/diffmatchpatch"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net/url"
	"os"
)

type PullCmd struct {
	env string
}

type PushCmd struct {
	env string
}

func (*PullCmd) Name() string     { return "pull" }
func (*PullCmd) Synopsis() string { return "pull config" }
func (*PullCmd) Usage() string {
	return `pull -env <environment>:
  Pull config into local file
`
}

func (*PushCmd) Name() string     { return "push" }
func (*PushCmd) Synopsis() string { return "push config" }
func (*PushCmd) Usage() string {
	return `push -env <environment>:
  Push local config into s3
`
}

func (p *PullCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.env, "env", os.Getenv("ENV"), "environment")
}

func (p *PushCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.env, "env", os.Getenv("ENV"), "environment")
}

func (p *PullCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if p.env == "" {
		f.Usage()
		return subcommands.ExitUsageError
	}

	cf := ctx.Value("cf").(*ConfigFile)
	config, err := getConfig(cf, p.env)
	if err != nil {
		panic(err)
	}

	s3, err := retrieveFile(config.Url, config.Region)
	if err != nil {
		panic(err)
	}

	err = ioutil.WriteFile(config.Local, s3, 0644)
	if err != nil {
		panic(err)
	}

	return subcommands.ExitSuccess
}

func (p *PushCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if p.env == "" {
		f.Usage()
		return subcommands.ExitUsageError
	}

	cf := ctx.Value("cf").(*ConfigFile)
	config, err := getConfig(cf, p.env)
	if err != nil {
		panic(err)
	}

	s3obj, err := retrieveFile(config.Url, config.Region)
	if err != nil {
		aerr := err.(awserr.Error)
		if aerr.Code() != s3.ErrCodeNoSuchKey {
			panic(err)
		}
	}

	local, err := ioutil.ReadFile(config.Local)
	if err != nil {
		panic(err)
	}

	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(string(s3obj), string(local), true)

	fmt.Println(dmp.DiffPrettyText(diffs))

	if !prompter.YN("Push?", true) {
		return subcommands.ExitFailure
	}

	err = putFile(config.Url, config.Region, config.Kms, local)
	if err != nil {
		panic(err)
	}

	return subcommands.ExitSuccess
}

type Config struct {
	Name   string
	Url    string
	Region string
	Local  string
	Kms    string
}

type ConfigFile struct {
	Enviroments []Config `yaml:"environments"`
}

func putFile(s3url string, region string, key string, data []byte) error {
	fragments, err := url.Parse(s3url)
	if err != nil {
		panic(err)
	}
	svc := s3.New(session.New(&aws.Config{Region: aws.String(region)}))
	_, err = svc.PutObject(&s3.PutObjectInput{
		Bucket:               aws.String(fragments.Host),
		Key:                  aws.String(fragments.Path),
		Body:                 bytes.NewReader(data),
		ServerSideEncryption: aws.String("aws:kms"),
		SSEKMSKeyId:          aws.String(key),
	})
	return err
}

func retrieveFile(s3url string, region string) ([]byte, error) {
	fragments, err := url.Parse(s3url)
	if err != nil {
		panic(err)
	}
	svc := s3.New(session.New(&aws.Config{Region: aws.String(region)}))
	params := &s3.GetObjectInput{Bucket: aws.String(fragments.Host), Key: aws.String(fragments.Path)}
	res, err := svc.GetObject(params)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()
	return ioutil.ReadAll(res.Body)
}

func getConfig(c *ConfigFile, key string) (*Config, error) {
	var config *Config

	for _, e := range c.Enviroments {
		if e.Name == key {
			config = &e
		}
	}

	if config == nil {
		err := fmt.Errorf("environment %s not found in config", key)
		return nil, err
	}

	return config, nil
}

func readConfigFile() *ConfigFile {
	data, err := ioutil.ReadFile(".s3-config.yaml")
	if err != nil {
		panic(err)
	}

	cf := ConfigFile{}
	err = yaml.Unmarshal(data, &cf)
	if err != nil {
		panic(err)
	}

	return &cf
}

func main() {
	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")
	subcommands.Register(&PullCmd{}, "")
	subcommands.Register(&PushCmd{}, "")

	flag.Parse()

	cf := readConfigFile()
	ctx := context.WithValue(context.Background(), "cf", cf)

	os.Exit(int(subcommands.Execute(ctx)))
}
