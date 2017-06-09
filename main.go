package main

import (
	"io/ioutil"
	"os"
	"text/template"
	"time"

	yaml "gopkg.in/yaml.v2"

	"strings"

	"github.com/pivotal-cf/om/api"
	"github.com/pivotal-cf/om/network"
)

type OpsMgrCredentials struct {
	Name        string `yaml:"name"`
	Username    string `yaml:"username"`
	Password    string `yaml:"password"`
	Target      string `yaml:"target"`
	ValidateSSL bool   `yaml:"validate_ssl"`
	Timeout     int    `yaml:"timeout"`
	IAAS        string `yaml:"iaas_type"`
}

type Config struct {
	OpsMgrs      []OpsMgrCredentials `yaml:"ops_managers"`
	PollInterval string              `yaml:"poll_interval"`
	PivnetToken  string              `yaml:"pivnet_token"`
	GithubToken  string              `yaml:"github_token"`
}

type TemplateInput struct {
	OpsMgr   OpsMgrCredentials
	Products []Product
	Config   Config
}

type Product struct {
	Name             string `yaml:"name"`
	Version          string `yaml:"version"`
	MetadataBasename string `yaml:"metadata_basename"`
	ProductSlug      string `yaml:"product_slug"`
}

func main() {
	var config Config

	config, err := ParseConfig(config)

	t, err := template.ParseFiles("./templates/update_tile.yml")
	if err != nil {
		panic(err)
	}

	for _, om := range config.OpsMgrs {
		input := TemplateInput{
			Products: GetOMProducts(om),
			OpsMgr:   om,
			Config:   config,
		}

		file, err := os.Create("upgrade-tile-" + om.Name + ".yml")
		if err != nil {
			panic(err)
		}
		defer file.Close()

		err = t.ExecuteTemplate(file, "update_tile.yml", input)
		if err != nil {
			panic(err)
		}

	}

}

func ParseConfig(config Config) (Config, error) {
	yamlFile, err := ioutil.ReadFile("./config.yml")
	if err != nil {
		return config, err
	}
	if err = yaml.Unmarshal(yamlFile, &config); err != nil {
		return config, err
	}
	return config, nil
}

func GetOMProducts(creds OpsMgrCredentials) []Product {
	products := GetStdProducts()

	requestTimeout := time.Duration(60) * time.Second

	authedClient, err := network.NewOAuthClient(creds.Target, creds.Username, creds.Password, "", "", !creds.ValidateSSL, false, requestTimeout)
	if err != nil {
		panic(err)
	}

	omProducts, err := api.NewStagedProductsService(authedClient).StagedProducts()
	if err != nil {
		panic(err)
	}

	var updateProducts []Product
	for _, prod := range omProducts.Products {
		newProd := GetProductInfo(prod.Type, products)
		if newProd != nil {
			newProd.Version = StarVersion(prod.ProductVersion)
			updateProducts = append(updateProducts, *newProd)
		}

	}

	return updateProducts
}

func GetStdProducts() []Product {
	var products []Product
	yamlFile, _ := ioutil.ReadFile("./products.yml")
	yaml.Unmarshal(yamlFile, &products)

	return products
}

func GetProductInfo(name string, products []Product) *Product {
	for _, prod := range products {
		if prod.Name == name {
			return &prod
		}
	}

	return nil
}

func StarVersion(version string) string {
	split := strings.Split(version, ".")
	if len(split) < 2 {
		return version
	}
	split[len(split)-1] = "*"
	return strings.Join(split, ".")
}
