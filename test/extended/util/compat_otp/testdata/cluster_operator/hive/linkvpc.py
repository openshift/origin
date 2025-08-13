def GenerateConfig(context):

    resources = [{
        'name': context.properties['name'] + '-link-network',
        'type': 'compute.v1.network',
        'properties': {
            'autoCreateSubnetworks': False,
            "routingConfig": {
                "routingMode": "GLOBAL"
            },
        }
    }, {
        'name': context.properties['name'] + '-link-subnet',
        'type': 'compute.v1.subnetwork',
        'properties': {
            'region': context.properties['region'],
            'network': '$(ref.' + context.properties['name'] + '-link-network.selfLink)',
            'ipCidrRange': context.properties['subnet_cidr']
        }
    }]

    return {'resources': resources}
