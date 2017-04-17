<!-- BEGIN MUNGE: UNVERSIONED_WARNING -->


<!-- END MUNGE: UNVERSIONED_WARNING -->

# What is Kubernetes?

Kubernetes is an open-source platform for automating deployment, scaling, and operations of application containers across clusters of hosts.

With Kubernetes, you are able to quickly and efficiently respond to customer demand:

 - Scale your applications on the fly.
 - Seamlessly roll out new features.
 - Optimize use of your hardware by using only the resources you need.

Our goal is to foster an ecosystem of components and tools that relieve the burden of running applications in public and private clouds.

#### Kubernetes is:

* **lean**: lightweight, simple, accessible
* **portable**: public, private, hybrid, multi-cloud
* **extensible**: modular, pluggable, hookable, composable
* **self-healing**: auto-placement, auto-restart, auto-replication

The Kubernetes project was started by Google in 2014. Kubernetes builds upon a [decade and a half of experience that Google has with running production workloads at scale](https://research.google.com/pubs/pub43438.html), combined with best-of-breed ideas and practices from the community.

##### Ready to [Get Started](getting-started-guides/README.md)?

<hr>

#### Why containers?

Looking for reasons why you should be using [containers](http://aucouranton.com/2014/06/13/linux-containers-parallels-lxc-openvz-docker-and-more/)?

Here are some key points:

* **Application-centric management**:
    Raises the level of abstraction from running an OS on virtual hardware to running an application on an OS using logical resources. This provides the simplicity of PaaS with the flexibility of IaaS and enables you to run much more than just [12-factor apps](http://12factor.net/).
* **Dev and Ops separation of concerns**:
    Provides separation of build and deployment; therefore, decoupling applications from infrastructure.
* **Agile application creation and deployment**:
    Increased ease and efficiency of container image creation compared to VM image use.
* **Continuous development, integration, and deployment**:
    Provides for reliable and frequent container image build and deployment with quick and easy rollbacks (due to image immutability).
* **Loosely coupled, distributed, elastic, liberated [micro-services](http://martinfowler.com/articles/microservices.html)**:
    Applications are broken into smaller, independent pieces and can be deployed and managed dynamically -- not a fat monolithic stack running on one big single-purpose machine.
* **Environmental consistency across development, testing, and production**:
    Runs the same on a laptop as it does in the cloud.
* **Cloud and OS distribution portability**:
    Runs on Ubuntu, RHEL, on-prem, or Google Container Engine, which makes sense for all environments: build, test, and production.
* **Resource isolation**:
    Predictable application performance.
* **Resource utilization**:
    High efficiency and density.

#### What can Kubernetes do?

Kubernetes can schedule and run application containers on clusters of physical or virtual machines.

It can also do much more than that.

Kubernetes satisfies a number of common needs of applications running in production, such as:
* [co-locating helper processes](user-guide/pods.md),
* [mounting storage systems](user-guide/volumes.md),
* [distributing secrets](user-guide/secrets.md),
* [application health checking](user-guide/production-pods.md#liveness-and-readiness-probes-aka-health-checks),
* [replicating application instances](user-guide/replication-controller.md),
* [horizontal auto-scaling](user-guide/horizontal-pod-autoscaler.md),
* [load balancing](user-guide/services.md),
* [rolling updates](user-guide/update-demo/), and
* [resource monitoring](user-guide/monitoring.md).

For more details, see the [user guide](user-guide/).

#### Why and how is Kubernetes a platform?

Even though Kubernetes provides a lot of functionality, there are always new scenarios that would benefit from new features. Ad hoc orchestration that is acceptable initially often requires robust automation at scale. Application-specific workflows can be streamlined to accelerate developer velocity. This is why Kubernetes was also designed to serve as a platform for building an ecosystem of components and tools to make it easier to deploy, scale, and manage applications.

[Labels](user-guide/labels.md) empower users to organize their resources however they please. [Annotations](user-guide/annotations.md) enable users to decorate resources with custom information to facilitate their workflows and provide an easy way for management tools to checkpoint state.

Additionally, the [Kubernetes control plane](admin/cluster-components.md) is built upon the same [APIs](api.md) that are available to developers and users. Users can write their own controllers, [schedulers](devel/scheduler.md), etc., if they choose, with [their own APIs](design/extending-api.md) that can be targeted by a general-purpose [command-line tool](user-guide/kubectl-overview.md).

This [design](design/principles.md) has enabled a number of other systems to build atop Kubernetes.

#### Kubernetes is not:

Kubernetes is not a PaaS (Platform as a Service).
* Kubernetes does not limit the types of applications supported. It does not dictate application frameworks, restrict the set of supported language runtimes, nor cater to only [12-factor applications](http://12factor.net/). Kubernetes aims to support an extremely diverse variety of workloads: if an application can run in a container, it should run great on Kubernetes.
* Kubernetes is unopinionated in the source-to-image space. It does not build your application. Continuous Integration (CI) workflow is an area where different users and projects have their own requirements and preferences, so we support layering CI workflows on Kubernetes but don't dictate how it should work.
* On the other hand, a number of PaaS systems run *on* Kubernetes, such as [Openshift](https://github.com/openshift/origin) and [Deis](http://deis.io/). You could also roll your own custom PaaS, integrate with a CI system of your choice, or get along just fine with just Kubernetes: bring your container images and deploy them on Kubernetes.
* Since Kubernetes operates at the application level rather than at just the hardware level, it provides some generally applicable features common to PaaS offerings, such as deployment, scaling, load balancing, logging, monitoring, etc. However, Kubernetes is not monolithic, and these default solutions are optional and pluggable.

Kubernetes is not a mere "orchestration system"; it eliminates the need for orchestration:
* The technical definition of "orchestration" is execution of a defined workflow: do A, then B, then C. In contrast, Kubernetes is comprised of a set of control processes that continuously drive current state towards the provided desired state. It shouldn't matter how you get from A to C: make it so. This results in a system that is easier to use and more powerful, robust, and resilient.

#### What does *Kubernetes* mean? K8s?

The name **Kubernetes** originates from Greek, meaning "helmsman" or "pilot", and is the root of "governor" and ["cybernetic"](http://www.etymonline.com/index.php?term=cybernetics). **K8s** is an abbreviation derived by replacing the 8 letters "ubernete" with 8.



<!-- BEGIN MUNGE: IS_VERSIONED -->
<!-- TAG IS_VERSIONED -->
<!-- END MUNGE: IS_VERSIONED -->


<!-- BEGIN MUNGE: GENERATED_ANALYTICS -->
[![Analytics](https://kubernetes-site.appspot.com/UA-36037335-10/GitHub/docs/whatisk8s.md?pixel)]()
<!-- END MUNGE: GENERATED_ANALYTICS -->
