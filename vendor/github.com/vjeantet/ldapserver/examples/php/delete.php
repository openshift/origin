<?php
$ldap_host = "ldap://127.0.0.1:10389";
$ldap_user  = "myLogin";
$ldap_pass = "pass";

//putenv('LDAPTLS_REQCERT=never');
ldap_set_option(NULL, LDAP_OPT_DEBUG_LEVEL, 0);

$ds = ldap_connect($ldap_host) or exit(">>Could not connect to LDAP server<<");
ldap_set_option($ds, LDAP_OPT_PROTOCOL_VERSION, 3);


ldap_start_tls($ds) ;

ldap_delete($ds, "cn=John Jones, o=My Company, c=US") ;