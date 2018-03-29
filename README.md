Go-based backend for the In/Out board. It uses Sqlite for a data store, and ldap for authentication.

Config file
----------------
The config holds the login credentials for the LDAP store. It looks like

~~~~
[Auth]
        Username=<ldapuser>
        BindPassword=<ldappassword>
        LdapPort=389
        LdapServer=<ldaphost>
        Realm=<ldaprealm>
        LdapSearchBase=<something like DC=Realm>
~~~~

The executable is dumb, so the config file should be in the current working directory when the server is
started.
