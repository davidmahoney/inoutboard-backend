package main

import ()

type Config struct {
	Auth struct {
		Username       string
		BindPassword   string
		LdapServer     string
		LdapPort       int
		Realm          string
		LdapSearchBase string
	}
}
