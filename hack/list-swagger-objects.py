import json;
import sys;
import string;

if len(sys.argv)!=2:
	print("Useage: python hack/list-swagger-objects.py <swagger-spec-location>")
	sys.exit(1)

swagger_spec_location=sys.argv[1]

with open(swagger_spec_location, 'r') as source:
	for model in json.load(source)["models"]:
		print(string.lower(model))