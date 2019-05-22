import boto3
import os
import sys
if len(sys.argv) < 3:
    print("Usage: ./update_route_53.py <RECORD> <IP>")
    sys.exit(1)
record = sys.argv[1]
ip = sys.argv[2]
print("record: %s" % record)
print("ip: %s" % ip)
domain = "%s.%s" % (os.environ["CLUSTER_NAME"], os.environ["BASE_DOMAIN"])
client = boto3.client('route53')
r = client.list_hosted_zones_by_name(DNSName=domain, MaxItems="1")
zone_id = r['HostedZones'][0]['Id'].split('/')[-1]
response = client.change_resource_record_sets(
    HostedZoneId=zone_id,
    ChangeBatch= {
        'Comment': 'add %s -> %s' % (record, ip),
        'Changes': [
        {
            'Action': 'UPSERT',
            'ResourceRecordSet': {
                'Name': record,
                'Type': 'A',
                'TTL': 60,
                'ResourceRecords': [{'Value': ip}]
            }
        }]
})
