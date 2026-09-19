# Diagrams

## How the services fit together

```mermaid
flowchart TB
    users([Customers])
    sites([Customer websites])

    subgraph cluster["Kubernetes cluster"]
        api["api - handles the web app"]
        sched["scheduler - decides what is due"]
        worker["worker - visits the websites"]
        notifier["notifier - sends alerts"]
        retention["retention - summarises old data"]
    end

    redis[("Redis - queue")]
    db[("PostgreSQL")]
    out([Email / Slack])

    users --> api
    api --> db
    sched --> redis
    sched --> db
    redis --> worker
    worker --> sites
    worker --> db
    notifier --> db
    notifier --> out
    retention --> db
```

## How it runs on AWS

```mermaid
flowchart TB
    internet([Internet])

    subgraph vpc["AWS VPC - ca-central-1"]
        subgraph public["Public subnets"]
            alb["Load Balancer"]
            nat["NAT gateway"]
        end
        subgraph private["Private subnets - no inbound"]
            node1["EKS node"]
            node2["EKS node"]
        end
        subgraph data["Data subnets - no internet at all"]
            rds[("RDS PostgreSQL")]
            cache[("ElastiCache Redis")]
        end
    end

    secrets["Secrets Manager"]
    ecr["ECR images"]

    internet --> alb
    alb --> node1
    alb --> node2
    node1 --> rds
    node2 --> rds
    node1 --> cache
    node2 --> cache
    node1 --> nat
    nat --> internet
    node1 --> secrets
    node1 --> ecr
```

The key point: the data subnets have no route to the internet in either
direction. Even if an attacker reached the database, there is no network path
out of it.
