FROM brew.registry.redhat.io/rh-osbs/openshift-ose-ansible-operator:vocpversion
COPY requirements.yml ${HOME}/requirements.yml
RUN ansible-galaxy collection install -r ${HOME}/requirements.yml --force \
 && chmod -R ug+rwx ${HOME}/.ansible

COPY watches.yaml ${HOME}/watches.yaml
COPY roles/ ${HOME}/roles/
COPY playbooks/ ${HOME}/playbooks/

COPY fixture_collection/ /tmp/fixture_collection/

USER root
RUN chmod -R ugo+rwx /tmp/fixture_collection
USER 1001

RUN ansible-galaxy collection build /tmp/fixture_collection/ --output-path /tmp/fixture_collection/ \
 && ansible-galaxy collection install /tmp/fixture_collection/operator_sdk-test_fixtures-0.0.0.tar.gz

USER root
RUN chmod -R ugo+rwx ${HOME}/.ansible
USER 1001

