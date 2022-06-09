# Snap cherry picker data

The snap cherry picker data periodically checks the current cherry picker cached data on the pocket regions and makes a copy of it to a database. Check the [deployment file](../serverless.yml) for the rate.

## Database fields

for more detailed data of the script, check the [script file](scripts/db-init.sql).

### cherry_picker_session

Is the aggregate data of all the regions of a session.

| Field            | Description                                                                                                                         |
|------------------|-------------------------------------------------------------------------------------------------------------------------------------|
| public_key       | Node's public key                                                                                                                   |
| chain            | Session's chain                                                                                                                     |
| session_key      | Session's key                                                                                                                       |
| session_height   | Session's height                                                                                                                    |
| address          | Node's address                                                                                                                      |
| total_success    | Aggregate of all the relay successes in all the regions                                                                             |
| total_failure    | Aggregate of all the relay failures in all the regions                                                                              |
| avg_success_time | Average time of all the successful relays on all the regions, is calculated with the `median_success_latency` field on every region |
| failure          | Whether the node got into failure (more than 5 failures in a row) at any point in any region                                        |

### cherry_picker_session_region

Individual cherry picker data of a session for a single region.

| Field                        | Description                                                                                                                               |
|------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------|
| public_key                   | Node's public key                                                                                                                         |
| chain                        | Session's chain                                                                                                                           |
| session_key                  | Session's key                                                                                                                             |
| session_height               | Session's height                                                                                                                          |
| region                       | region from where the cherry picker data is being extracted                                                                               |
| address                      | Node's address                                                                                                                            |
| total_success                | Sum of all the relay successes for the region                                                                                             |
| total_failure                | Sum of all the relay failures for the region                                                                                              |
| median_success_latency       | median success latency of all the relays being used for weighting, each value is the result of the snapshot at the time                   |
| weighted_success_latency     | weighted success latency of all the relays being used for weighting, each value is the result of the snapshot at the time                 |
| avg_success_latency          | Average of all the median success latency saved so far                                                                                    |
| avg_weighted_success_latency | Average of all the weighted success latency saved so far                                                                                  |
| p_90_latency                 | p90 success latency of all the relays being used for weighting, each value is the result of the snapshot at the time                      |
| attempts                     | Amount of relays used for weighting, this includes the successful and failed relays, each value is the result of the snapshot at the time |
| success_rate                 | Success rate of all the relays weigthed at the moment, used for bucketing. Value is calculated as taking a sample of all the relays made for the node in the session and calculating as `totalSuccessRelays/totalAttemptsMade`. Each value is the result of the snapshot at the time                                                                |
| failure                      | Whether the node got into failure (more than 5 failures in a row) at any point in the region                                              |
