# Fixture scrubber

Generate repository fixtures from raw Slurm JSON dumps with:

```sh
go run ./tools/scrubfixtures -input /private/path/squeue_all.json -output testdata/squeue.json -max-jobs 20
```

The scrubber puts pending jobs first when trimming, then keeps the original
order within each state group. `-offset` and `-count` can split a selected job
set into multiple compact JSON fixtures; scrub the same full set for each
piece so anonymized values remain consistent. Use `-omit-job-resources` for
queue fixtures that do not need the large allocation details.

It replaces user and group names and IDs, accounts, work keys, job names, site
labels, node names, paths, scripts, command lines, comments, and free-form
extra text with deterministic fixture values. Keep raw dumps outside the
repository; only commit the scrubber's output.
