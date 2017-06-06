#Operations

##Get pod

```
GET /capacity/pod
```

###Parameters
| Type           | Name   | Description                                | Required | Schema | Default |
|----------------|--------|--------------------------------------------|----------|--------|---------|
| QueryParameter | pretty | If true, then the output is pretty printed | false    | string |         |

###Responses
| HTTP Code | Description | Schema            |
|-----------|-------------|-------------------|
| 200       | success     | kubernetes v1.pod |

##Update pod

```
POST /capacity/pod
```

###Parameters
| Type          | Name | Description                  | Required | Schema            | Default |
|---------------|------|------------------------------|----------|-------------------|---------|
| BodyParameter | body | May be in json on yaml form. | true     | kubernetes v1.pod |         |

###Responses
| HTTP Code | Description | Schema            |
|-----------|-------------|-------------------|
| 200       | success     | kubernetes v1.pod |

##Get status

```
GET /capacity/status
```

###Parameters
| Type           | Name  | Description                                                           | Required | Schema  | Default                   |
|----------------|-------|-----------------------------------------------------------------------|----------|---------|---------------------------|
| QueryParameter | num   | Number of records to be shown. If not set, all records are listed     | false    | number  |                           |
| QueryParameter | watch | Watch for new statuses.                                               | false    | boolean |                           |
| QueryParameter | since | Time in RFC 3339 form. Do not list records acquired before this time. | false    | time    | 1970-01-01T00:00:00+00:00 |
| QueryParameter | to    | Time in RFC 3339 form. Do not list records acquired after this time.  | false    | time    | time.now                  |

###Responses
| HTTP Code | Description | Schema       |
|-----------|-------------|--------------|
| 200       | success     | Report array |