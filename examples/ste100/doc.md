# Deployment Notes

It should be noted that, in the event that the deployment pipeline has been
configured in such a manner whereby the aforementioned containerized artifacts
are, prior to their ultimate promotion into the production environment, subjected
to a comprehensive battery of automated validation procedures, the operations
team, which is responsible for the maintenance and ongoing operational integrity
of the platform, may, at their sole discretion and without any obligation to
provide advance notification to the stakeholders, elect to roll back, either
wholly or partially, the release in question. Furthermore, any such rollback
activity, should it be deemed necessary by the aforementioned personnel, will
have been documented in the incident management system by the responsible
individuals, who shall endeavour, to the best of their ability, to ensure that
the root cause analysis is completed and circulated amongst the relevant parties
in a timely fashion.

You'll want to, like, totally make sure the config's good before you push it,
otherwise stuff might blow up in prod, which is a huge bummer for everyone
involved. So yeah, just kind of double-check it and hope for the best, because
nobody really knows what's going on in there half the time.
