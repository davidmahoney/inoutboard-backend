Go-based backend for the In/Out board. It uses Sqlite for a data store, and ldap for authentication.

Config file
----------------
The config holds the login credentials for the LDAP store, and other settings. It looks like

~~~~
[Auth]
        Username=<ldapuser>
        BindPassword=<ldappassword>
        LdapPort=389
        LdapServer=<ldaphost>
        Realm=<ldaprealm>
        LdapSearchBase=<something like DC=Realm>

[Files]
	StaticFilesPath=<path to static files dir>
	DbPath=<path to database file (it will be created if it doesn't exist)>

[Net]
	Port=<listen port>
~~~~

Environment Variables
--------------------------
-INOUTBOARD\_CONFIG: a path to the actual config file.
-INOUTBOARD\_STATIC: a path to the static files directory.

Installation
--------------------------

### Systemd-based Linux Systems

1. Copy the `inoutboard.service` file to `/etc/systemd/system`.
2. Create the `/etc/inoutboard.d` directory.
3. Copy the config file and TLS keys to `/etc/inoutboard.d`
4. Copy the static files to the static files directory (as listed in the config file)
5. Cross your fingers and start the server with `systemd start inoutboard`
6. Check the server log with `journalctl -u inoutboard`
