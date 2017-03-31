# DC/OS Auto-scaling

An autoscaler for mesospheres [DC/OS](https://dcos.io/). Works by parsing special marathon application labels which indicate whether the app needs to be auto scaled, the rules that needs to be used for auto scaling, metadata that describes how the application needs to be scaled.

Marathon's v2 api is used to collect data about the running apps. CPU and memory usage are aquired using mesos statistics api where authentication is bypassed given that the slave running this application has access to a mesos dns and can connect to all slaves.

## Usage
Run this app in your DC/OS either by directly installing it from the universe or by using this marathon json.
```
{
  "id": "/dcos-autoscaler",
  "instances": 1,
  "cpus": 0.5,
  "mem": 500,
  "container": {
    "docker": {
      "image": "ahmadposten/dcos-autoscaling"
    }
  }
}
```

You would need to label your application as autoscalable by adding this label to your application
`AUTOSCALABLE=true`

Specify the minimum and maximum number of instances by
`AUTOSCALING_MIN_INSTANCES=<minumum number>` and `AUTOSCALING_MAX_INSTANCES=<maximum number>`

Rules are specified by adding labels in the format of

`AUTOSCALING_{Rule number}_RULE_{property}`

for example If you wish to scale your application up based on cpu utilization you add the following labels to your application
```
{
  "AUTOSCALABLE": "true",
  "AUTOSCALING_MIN_INSTANCES": "1",
  "AUTOSCALING_MAX_INSTANCES": "10",
  "AUTOSCALING_COOLDOWN_PERIOD": "60",
  "AUTOSCALING_0_RULE_TYPE": "cpu",
  "AUTOSCALING_0_RULE_THRESHOLD": "70",
  "AUTOSCALING_0_RULE_STEP": "2",
  "AUTOSCALING_0_RULE_OPERATOR": "gt",
  "AUTOSCALING_0_RULE_SAMPLES": 5,
  "AUTOSCALING_0_RULE_INTERVAL": 60,
  "AUTOSCALING_0_RULE_ACTION": "increase"
}
```

By these labels I defined that this application will be scalable, it can never go below 1 instance or above 10 instances. After every scaling activity wait for 1 minute before doing another one if needed. If the cpu utilization exceeds 70 for 5 samples of 1 minutes each increase the instances by 2 instances.


## Supported labels

| Label                          | Description                                                                                                                     | Scope       | Possible values    |
|--------------------------------|---------------------------------------------------------------------------------------------------------------------------------|-------------|--------------------|
| AUTOSCALABLE                   | Specifies that your application is supervised by the autoscaler                                                                 | Application | Boolean            |
| AUTOSCALING_MIN_INSTANCES      | Specifies the minimum number of instances that your app need to maintain                                                        | Application | Integer            |
| AUTOSCALING_MAX_INSTANCES      | Specifies the minimum number of instances that your app need to maintain                                                        | Application | Integer            |
| AUTOSCALING_COOLDOWN_PERIOD    | The number of seconds to wait before going on with another scaling activity                                                     | Application | Integer            |
| AUTOSCALING_{X}_RULE_TYPE      | The type of the rule x                                                                                                          | Rule        | cpu, memory        |
| AUTOSCALING_{X}_RULE_THRESHOLD | The threshold of rule x                                                                                                         | Rule        | Double             |
| AUTOSCALING_{X}_RULE_OPERATOR  | The comparison operator (> or <) against the threshold                                                                          | Rule        | gt, lt             |
| AUTOSCALING_{X}_RULE_INTERVAL  | The interval every measurement will be taken over in seconds                                                                    | Rule        | Integer            |
| AUTOSCALING_{X}_RULE_SAMPLES   | The number of consecutive samples to average  and compare to the threshold.  Every sample is taken over the specified interval. | Rule        | Integer            |
| AUTOSCALING_{X}_RULE_ACTION    | The action to take when the rule is triggered.                                                                                  | Rule        | increase, decrease |
| AUTOSCALING_{X}_RULE_STEP    | The step to scale by when rule is triggered.                                                                                  | Rule        | Integer |


## Features
- Scale an application up and down based on:
  - CPU
  - Memory
- Configurable number of samples
- Configurable interval
- Configurable Operators (gt, lt)
- Configurable cooldown period (the number of seconds to wait before scaling again)
- Configurable min and max number of instances
- Supports multiple rules

## TODO
The project is still not stable and lot's of stuff can be done to improve it and make it production ready. Here are some of the things I was thinking about
- Support scaling based on number of requests
- Support scaling based on network in/out
- Support notifications

and most importantly testing.

## Contributing
Everyones contribution is very welcomed. You can pick one item from the TODO list and work on it or you can create an issue if you found a bug or propose an improvement to be made. To actively contribute code to this project:
1. Fork the repo
2. Do you work and make it ready on your fork
3. Submit a pull request

Since this is a new project and it is not actively used (or contributed to except by me) expect responses to your pull requests to be really quick.

## License
This source code is distributed under the BSD-3-Clause. Read the [License File](LICENSE.md)


