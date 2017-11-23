import getpass
import ldap
import logging
import logging.config
import optparse
import os
import os.path

logger = logging.getLogger('abandon')


def test_abandon(uri, binddn, passwd, bases, timeout, retries, filterstr,
                 *attributes):
    """
    Execute a deliberately long-running query using asynchronous LDAP
    operations.
    """
    l = ldap.initialize(uri)
    l.set_option(ldap.OPT_DEREF, ldap.DEREF_ALWAYS)

    b = l.simple_bind(binddn, passwd)
    br = l.result(b, timeout=10)
    logger.debug('simple_bind="%d", result_type="%s", result_data="%r"',
                 b, *br)

    # if there were no attributes explicitly requested, reset to None
    if not attributes:
        attributes = None

    for r in xrange(retries):
        for base in bases:
            msgid = l.search_ext(base, ldap.SCOPE_ONELEVEL, filterstr,
                                 attributes, timeout=timeout)
            logger.debug('search="%d", base="%s", timeout="%d", filter="%s", '
                         'attributes="%s"',
                         msgid, base, timeout, filterstr, attributes)

            try:
                res = l.result(msgid, timeout=timeout)
            except ldap.TIMEOUT:
                logger.error('abandon="%d"', msgid)
                l.abandon(msgid)
            except ldap.TIMELIMIT_EXCEEDED:
                logger.error('timelimit="%d"', msgid)
            except ldap.LDAPError:
                logger.exception('error="%d"', msgid)
            else:
                logger.debug('result_type="%s", result_data="%r"', *res)


if __name__ == "__main__":
    parser = optparse.OptionParser()

    parser.add_option('-H', dest='ldapuri', default="ldap://127.0.0.1:10389")
    parser.add_option('-D', dest='binddn', default="login")
    parser.add_option('-w', dest='passwd', default="pass")
    parser.add_option('-b', dest='searchbase', action='append')
    parser.add_option('-l', dest='logconfig', default='abandon.ini')
    parser.add_option('-t', dest='timeout', type="int", default=1)
    parser.add_option('-r', dest='retries', type="int", default=1)
    parser.add_option('-f', dest='filterstr', type="string", default='*')

    opts, args = parser.parse_args()

    ini = os.path.abspath(opts.logconfig)
    logging.config.fileConfig(ini, disable_existing_loggers=False)

    passwd = opts.passwd or os.environ.get('LDAP_PASSWD') or getpass.getpass()
    test_abandon(opts.ldapuri,
                 opts.binddn,
                 passwd,
                 opts.searchbase,
                 opts.timeout,
                 opts.retries,
                 opts.filterstr,    
                 *args)