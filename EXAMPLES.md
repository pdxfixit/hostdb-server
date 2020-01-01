# Using the HostDB service

Aside from the GUI, there are three main API endpoints which are versioned (currently at `v0`). The URL pattern is:

* `https://hostdb.pdxfixit.com/v0/detail/`

  The `detail` endpoint is for getting back as much data as possible about a set of records.

* `https://hostdb.pdxfixit.com/v0/list/`

  The `list` endpoint is for displaying a brief list.

* `https://hostdb.pdxfixit.com/v0/records/`

  Finally, the `records` endpoint is for interacting with individual records, and write operations.

For the major browser user-agents (Chrome, Mozilla/Firefox/Gecko, Edge), pretty-printed JSON will be returned.
All others will receive compacted JSON.

## Searching

There are two ways to search.
First is a "simple" search which attempts to find a text match just about anywhere in a record, similar to `grep`.
The second is using individual arguments and stringing them together.

### Simple

The "simple" search is accomplished by using the key `_search` like so:

  ```bash
  $ curl https://hostdb.pdxfixit.com/v0/list/?_search=stomp
  ```

### Parameters

The other method of searching, is by using query parameters.
One searches by specifying a field as the key, and the value, like so: `ip=10.20.30.40`

Parameters can be appended like so: `datacenter=va2&ip=10.20.30.40`
(linux shells interpret `&` as a special character, so be sure to quote URLs)

* Get all OpenStack hosts in the tenant `webapp`

  ```bash
  $ curl "https://hostdb.pdxfixit.com/v0/list/?type=openstack&tenant=webapp"
  ```

#### Multiple values

Multiple, comma separated values can be given like this:


  ```bash
  $ curl "https://hostdb.pdxfixit.com/v0/list/?type=openstack&tenant=webapp,other"
  ```

#### Regex patterns

PCRE patterns are supported, by wrapping the pattern in forward slashes, like in the example below.

* Look for hosts with a partial IP address

  ```bash
  $ curl https://hostdb.pdxfixit.com/v0/list/?ip=/10\.20/
  ```

#### NOT operator

If one wishes to make an argument negative, prefix the key with a bang, like so: `!ip` or `!_search`.
This will add the argument as a NOT or negative assertion.

* Look for VMs without an IP address

  ```bash
  $ curl "https://hostdb.pdxfixit.com/v0/list/?type=vrops-vmware-virtualmachine&!ip=/./"
  ```

#### Reference

Below is a list of some of the available fields to search within.
A complete list can be found in the [API configuration](https://hostdb.pdxfixit.com/v0/config) (see the `QueryParams` element).
Each of these fields can map to a different data location in each `type` of record. Reach out if you have questions.

[//]: # (fields are going to bear a relationship to type, b/c each type will have different data fields)

* `app`
* `aws-account-id`
* `aws-region`
* `datacenter`
* `description`
* `env`
* `flavor`
* `flavor_id`
* `hostname`
* `id`
* `image`
* `ip`
* `owner`
* `puppet`
* `sin`
* `stack`
* `status`
* `tenant`
* `tenant_id`
* `type`
* `ucs_url`
* `vc_name`
* `vc_url`

## Pagination

By default, 100 records will be returned in a result set.
The number of records returned can be specified by using the `_limit` argument.
Paired with `_offset`, pagination can be achieved.
In the below example, we retrieve 3 records, starting with record number 7.

  ```bash
  $ curl "https://hostdb.pdxfixit.com/v0/list/?owner=/ben/&_limit=3&_offset=6"
  ```

## Custom List Output

Because the `list` endpoint returns just a few fields for each record by default, it also supports the ability to specify which fields to display.
Use the `_fields` query parameter to provide a comma delimited list of fields to return.

  ```bash
  $ curl https://hostdb.pdxfixit.com/v0/list/?_fields=hostname,context
  ```

Currently, only the following fields can be displayed from the `list` endpoint with the `_fields` parameter:

* `id`
* `type`
* `timestamp`
* `committer`
* `hostname`
* `ip`
* `context`
* `data`
* `hash`

## Catalog

The catalog endpoint will return all unique values for a given field, such as `type` in the example below.

  ```bash
  $ curl https://hostdb.pdxfixit.com/v0/catalog/type
  ```

There are query parameter options to display frequency count (`count`) and list filtering with a regex pattern (`filter`).
Examples below.

### Catalog Frequency Count

Add the query parameter `count` to learn how many occurrences of each value are present.

  ```bash
  $ curl https://hostdb.pdxfixit.com/v0/catalog/type?count
  ```

### Catalog Filtering

Use the `filter` parameter (with a regex pattern) to get a subset of catalog values.

  ```bash
  $ curl https://hostdb.pdxfixit.com/v0/catalog/type?filter=/aws/
  ```

## CRUD

* Create a record with an ID of `abc123`

  ```bash
  $ curl -X PUT -H "Authorization: Basic ymmv=" \
          -H "Content-Type: application/json" \
          -d '{"type":"test","hostname":"foobar.pdxfixit.com","ip":"10.20.30.40","timestamp":"0000-00-00 00:00:00","committer":"test","data":"wahoo","context":{"test":true}}' \
          https://hostdb.pdxfixit.com/v0/records/abc123
  ```
  ```json
  {
    "id": "abc123",
    "ok": true
  }
  ```

* Get a record

  ```bash
  $ curl https://hostdb.pdxfixit.com/v0/records/abc123
  ```
  ```json
  {
      "count": 1,
      "query_time": "1.824845ms",
      "records": {
          "abc123": {
              "id": "abc123",
              "type": "test",
              "hostname": "foobar.pdxfixit.com",
              "ip": "10.20.30.40",
              "timestamp": "0000-00-00 00:00:00",
              "committer": "test",
              "context": {
                  "test": true
              },
              "data": "wahoo",
              "hash": "abc123"
          }
      }
  }
  ```

* Update a record

  ```bash
  $ curl -X PUT -H "Authorization: Basic ymmv=" \
          -H "Content-Type: application/json" \
          -d '{"type":"test","hostname":"foobar.pdxfixit.com","ip":"10.20.30.40","timestamp":"1999-12-31 23:59:59","committer":"testing","data":"wahoo!","context":{"test":true}}' \
          https://hostdb.pdxfixit.com/v0/records/abc123
  ```
  ```json
  {
    "id": "abc123",
    "ok": true
  }
  ```

* Delete a record

  ```bash
  $ curl -X DELETE -H "Authorization: Basic ymmv=" \
          https://hostdb.pdxfixit.com/v0/records/abc123
  ```
  ```json
  {
    "deleted": true,
    "id": "abc123"
  }
  ```

## Bulk add records

When records are being added in bulk, the assumption is that the entire state (or set of records) are being provided in the request.

A bulk record request should be associated with a single `type`.
Any records not in the request for the provided `type` are considered stale, and will be deleted.
Certain types have special handling to consider additional elements in the `context`.
Please reach out if you have questions.

**NOTE: Upon receiving records, any records which were in the database for the given `type`, but not in the request, are considered stale and will be deleted.** 

The bulk API body should be JSON, and contain the following fields:
  
  * `type`
  * `timestamp` (optional)
  * `context` (an optional hash of contextual info)
  * `committer` (optional)
  * `records` (an array of records)
    * `context` (an optional hash which will get merged with (overwriting) the bulk context above)
    * `data` (the important metadata to be stored)

  ```json
  {
    "type": "openstack",
    "timestamp": "1999-12-31 23:59:59",
    "context": {
      "datacenter": "sj",
      "os_auth_url": "https://openstack.pdxfixit.com:5000/v2.0",
      "stack_name": "sj_s1",
      "tenant_name": "webapp",
      "tenant_id": "a1b2c3"
    },
    "committer": "Testy McTesterton",
    "records": [
      {
        "hostname": "foo.pdxfixit.com",
        "ip": "1.1.1.1",
        "data":
          {
            "test": "foo"
          }
      },
      {
        "hostname": "bar.pdxfixit.com",
        "ip": "2.2.2.2",
        "data":
          {
            "test": "bar"
          }
      },
      {
        "context": {
          "special": true
        },
        "hostname": "baz.pdxfixit.com",
        "ip": "3.3.3.3",
        "data":
          {
            "test": "baz"
          }
      }
    ]
  }
  ```

  ```bash
  $ curl -X POST -H "Authorization: Basic ymmv=" \
          -H "Content-Type: application/json" \
          -d @example.json \
          https://hostdb.pdxfixit.com/v0/records/
  ```

  The response will be a simple ok: true/false, with an error message (if applicable).

  ```json
  {
    "ok": true,
    "error": ""
  }
  ```

## CSV Output

The endpoint `/v0/csv/` can be used with the query parameters described above, to receive a CSV file of results.

```bash
$ curl -JLOs https://hostdb.pdxfixit.com/v0/csv/?owner=/ben/
```

## Advanced Usage

Sometimes one may want to perform action operations on a set of servers. The pairing of HostDB and the utility [jq](https://stedolan.github.io/jq/) can be very useful in this scenario.

Consider the following example, where we wish to update a tagged owner in OpenStack.

  * First, we use curl to request a list of hosts.
  * Then, we parse the response, and extract the data points we need (in this case, `tenant_name` and `id`).
  * Finally, we use `while read` to loop over each record and execute `nova` (via the OpenStack CLI wrapper `osw`) to update each host's metadata.

  ```bash
  $ curl -s https://hostdb.pdxfixit.com/v0/detail/?_search=tools | \
    jq -r '.records[] | "\(.context.tenant_name) \(.data.id)"' | \
    while read tenant id; do osw $tenant nova meta $id set owner.list=alice,ben,charles; done
  ```

## Other tasks

* Health check

  Returns the state of the app, and the availability of the database.
  A db status of `missing` means that a connection to the database cannot be established.

  ```bash
  $ curl https://hostdb.pdxfixit.com/health
  ```

* Version information

  ```bash
  $ curl https://hostdb.pdxfixit.com/version
  ```

* Current API configuration

  ```bash
  $ curl https://hostdb.pdxfixit.com/v0/config
  ```

* Current system configuration (requires admin privileges)

  ```bash
  $ curl -H "Authorization: Basic ymmv=" https://hostdb.pdxfixit.com/admin/showConfig
  ```
