## Future Potential Work Items

- **Recipe Management**:
  Perhaps have a DockerHub-like system where people can download/install
  entire applications w/o having to create the deployment yaml/json themselves.

- **Built-In Service Broker**:
  Provide a default Service Broker so that service providers do not need
  to implement the CF SB APIs themselves - they would just need to support
  the minimal back-end APIs, if any.  We may simply provide the SB a pointer
  to a recipe instead.
