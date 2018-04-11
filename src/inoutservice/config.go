package main

import ()

type Config struct {
	Net struct {
		Port int
	}

	Auth struct {
		Username       string
		BindPassword   string
		LdapServer     string
		LdapPort       int
		Realm          string
		LdapSearchBase string
	}

	Files struct {
		StaticFilesPath string
		DbPath          string
	}
}
