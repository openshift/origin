FROM brew.registry.redhat.io/rh-osbs/openshift-ose-ansible-operator:vocpversion

COPY requirements.yml ${HOME}/requirements.yml
RUN ansible-galaxy collection install -r ${HOME}/requirements.yml --force \
 && chmod -R ug+rwx ${HOME}/.ansible
RUN ansible-galaxy collection install operator_sdk.util --force \ 
 && chmod -R ug+rwx ${HOME}/.ansible

COPY watches.yaml ${HOME}/watches.yaml
COPY roles/ ${HOME}/roles/
COPY playbooks/ ${HOME}/playbooks/
