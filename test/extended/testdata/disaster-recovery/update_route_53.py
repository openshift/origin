import boto3
import os
import sys
from time import sleep

if len(sys.argv) < 4:
    print("Usage: ./update_route_53.py <DOMAIN> <RECORD> <IP>")
    sys.exit(1)

attempts = 10
pause = 10

domain = sys.argv[1]
record = sys.argv[2]
ip = sys.argv[3]
print("record: %s" % record)
print("ip: %s" % ip)

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
for i in range(attempts):
    print('response: %s' % response)
    changeID = response['ChangeInfo']['Id']
    if response['ChangeInfo']['Status'] == "INSYNC":
        print('insync found, response: %s' % response)
        break
    print('waiting for response to complete')
    sleep(pause)
    response = client.get_change(Id=changeID)
