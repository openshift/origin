#
# This is a mock pod used to facilitate inter/intra communication 
# It contains a mockserver that has usefull primitives as REST API
#
# - /name              : return the "functional name" of the mockpod             
# - /put/{key}/{value} : put a value for a key                                   
# - /get/{key}         : get a value for gey . send error if key is not present. 
# - /contains/{key}    : tell if it contains a given key                         
# - /send/{message}    : will send a get message another pod via an url.         
# - /listkeys          : will list all the keys                                  
# - /kill              : will terminate the server so the pod.
# 
# The standard name for this image is openshift/mock-app
#
FROM scratch

ADD bin/mock-app /mock-app

EXPOSE 8080

CMD ["/mock-app" , "-logtostderr"]
