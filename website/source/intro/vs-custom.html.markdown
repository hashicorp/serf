---
layout: "intro"
page_title: "Serf vs. Custom Solutions"
sidebar_current: "vs-other"
---

# Serf vs. Custom Solutions

Many organizations find themselves building home grown solutions
for service discovery and administration. It is an undisputed fact that
distributed systems are hard; building one is error prone and time consuming.
Most systems cut corners by introducing single points of failure such
as a single Redis or RDBMS to maintain clusterstate. These solutions may work in the short term,
but usually they are not fault tolerant or scalable. Besides these limitations,
they require time and resources to build and maintain.

Serf may not provide the exact feature set needed by an organization,
but it can be used as building block. It provides generally useful features
that are can be used for building distributed systems, and requires no
effort to use out of the box.

Serf is built on top of well-cited academic research where the pros, cons,
failure scenarios, scalability, etc. are all well defined.
