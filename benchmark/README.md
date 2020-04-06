# Benchmark

Running the benchmark yourself:

```
go test -run=^$ -bench=. -benchmem
```

## Results

Platform: `Go 1.14.1 | i7 4770K (Haswell; 4 physical, 8 logical cores) @ 4.4GHz | Win 10`, ran on `2020/04/06`.

All libraries being compared are listed as ➜ [Alternatives](./README.md#alternatives) in the root package.

<br />

### Generation

These results must **not** be taken for their raw numbers. See the explanation 
(primarily about the `unbounded` suffix) afterwards.

**Sequential**
```
sno/unbounded   136208883           8.80 ns/op      0 B/op          0 allocs/op
xid              59964620          19.4 ns/op       0 B/op          0 allocs/op
uuid/v1          33327685          36.3 ns/op       0 B/op          0 allocs/op
ulid/math        23083492          50.3 ns/op      16 B/op          1 allocs/op
sno/bounded      21022425          61.0 ns/op       0 B/op          0 allocs/op
ulid/crypto       5797293         204 ns/op        16 B/op          1 allocs/op
uuid/v4           5660026         205 ns/op        16 B/op          1 allocs/op
ksuid             5430244         206 ns/op         0 B/op          0 allocs/op
sandflake         5427452         224 ns/op         3 B/op          1 allocs/op
snowflake         4917784         244 ns/op         0 B/op          0 allocs/op
cuid              3507404         342 ns/op        55 B/op          4 allocs/op
sonyflake           31000       38938 ns/op         0 B/op          0 allocs/op
```

**Parallel** (8 threads)

```
sno/unbounded    65161461          17.8 ns/op       0 B/op          0 allocs/op
xid              63163545          18.1 ns/op       0 B/op          0 allocs/op
sno/bounded      21022425          61.0 ns/op       0 B/op          0 allocs/op
uuid/v1           8695777         137 ns/op         0 B/op          0 allocs/op
uuid/v4           7947076         151 ns/op        16 B/op          1 allocs/op
ulid/crypto       7947030         151 ns/op        16 B/op          1 allocs/op
sandflake         6521745         184 ns/op         3 B/op          1 allocs/op
ulid/math         5825053         206 ns/op        16 B/op          1 allocs/op
snowflake         4917774         244 ns/op         0 B/op          0 allocs/op
ksuid             3692324         316 ns/op         0 B/op          0 allocs/op
cuid              3200022         371 ns/op        55 B/op          4 allocs/op
sonyflake           30896       38740 ns/op         0 B/op          0 allocs/op
```

**Snowflakes**

What does `unbounded` mean? [xid], for example, is unbounded, i.e. it does not prevent you from generating more IDs 
than it has a pool for (nor does it account for time). In other words - at high enough throughput you simply and 
silently start overwriting already generated IDs. *Realistically* you are not going to fill its pool of 
16,777,216 items per second. But it does reflect in synthetic benchmarks. [Sandflake] does not bind nor handle clock 
drifts either. In both cases their results are `WYSIWYG`.

The implementations that do bind, approach this issue (and clock drifts) differently. [Sonyflake] goes to sleep, 
[Snowflake] spins aggressively to get the OS time. **sno**, when about to overflow, starts a single timer and 
locks all overflowing requests on a condition, waking them up when the sequence resets, i.e. time changes.

Both of the above are edge cases, *realistically* - Go's benchmarks happen to saturate the capacities and hit 
those cases. **Most of the time what you get is the unbounded overhead**.  Expect said overhead of the 
implementations to be considerably lower, but still higher than [xid] and **sno** due to their locking nature. 
Similarily, expect some of the generation calls to **sno** to be considerably *slower* when they drop into an 
edge case branch, but still very much in the same logarithmic ballpark.

Note: the `61.0ns/op` is our **throughput upper bound** - `1s / 61.0 ns`, yields `16,393,442`. It's an imprecise 
measure, but it actually reflects `16,384,000` - our pool per second. If you shrink that capacity using custom 
sequence bounds, that number - `61.0ns/op` - will start growing exponentially, but only if/as your burst through 
the available capacity.

[Sonyflake], for example, is limited to 256 IDs per 10msec (25 600 per second), which is why its numbers *appear* so 
high - and why the comparison has a disclaimer.

**`sno/unbounded`**

In order to get the `unbounded` results in **sno**'s case, `Generator.New()` must be modified locally
and the...
```
if g.seqMax >= seq {...}
```
...condition removed.

**Entropy**

All entropy-based implementations lock - and will naturally be slower as they need to read from a entropy source and 
have more bits to fiddle with. [ULID] implementation required manual locking of rand.Reader for the parallel test.


<br /><br />

### Encoding/decoding

The comparisons below are preceded by some baseline measures for sno relative to std's base32 package
as a reference.

- `sno/vector` - amd64 SIMD code,
- `sno/scalar` - assembly based fallback on amd64 without SIMD,
- `sno/pure-go` - non-assembly, pure Go implementation used by sno on non-amd64 platforms.

The actual comparison results utilized `sno/vector` in our case, but `sno/pure-go`  - albeit slower -
places just as high.

**Excluded**
- [Sonyflake] has no canonical encoding;
- [cuid] is base36 only (no binary representation);

**Notes**
- Expect JSON (un)marshal performance to be nearly identical in most if not all cases;


#### Encoding

**Baseline**
```
sno/vector   2000000000       0.85 ns/op        0 B/op        0 allocs/op
sno/scalar   1000000000       2.21 ns/op        0 B/op        0 allocs/op
sno/pure-go  1000000000       2.70 ns/op        0 B/op        0 allocs/op
std          30000000        12.5 ns/op         0 B/op        0 allocs/op
```

**Comparison**

```
sno          963900753        1.18 ns/op        0 B/op        0 allocs/op
xid          240481202        4.94 ns/op        0 B/op        0 allocs/op
ulid         211640920        5.67 ns/op        0 B/op        0 allocs/op
snowflake    71941237        16.5 ns/op        32 B/op        1 allocs/op
sandflake    58868926        21.5 ns/op        32 B/op        1 allocs/op
uuid/v4      55494362        22.1 ns/op        48 B/op        1 allocs/op
uuid/v1      51785808        22.2 ns/op        48 B/op        1 allocs/op
ksuid        19672356        54.7 ns/op         0 B/op        0 allocs/op
```

Using: `String()`, provided by all packages.


#### Decoding

**Baseline**
```
sno/vector   2000000000       1.02 ns/op        0 B/op        0 allocs/op
sno/scalar   500000000        2.41 ns/op        0 B/op        0 allocs/op
sno/pure-go  500000000        2.79 ns/op        0 B/op        0 allocs/op
std          50000000        31.8 ns/op         0 B/op        0 allocs/op
```

**Comparison**

```
sno          863313699        1.30 ns/op        0 B/op        0 allocs/op
ulid         239884370        4.98 ns/op        0 B/op        0 allocs/op
xid          156291760        7.62 ns/op        0 B/op        0 allocs/op
snowflake    127603538        9.32 ns/op        0 B/op        0 allocs/op
uuid/v1      30000150        35.7 ns/op        48 B/op        1 allocs/op
uuid/v4      30000300        35.7 ns/op        48 B/op        1 allocs/op
ksuid        27908728        37.5 ns/op         0 B/op        0 allocs/op
sandflake    25533001        40.6 ns/op        32 B/op        1 allocs/op
```

Using: `sno.FromEncodedString`, `ulid.Parse`, `xid.FromString`, `snowflake.ParseString`, `sandflake.Parse`, `ksuid.Parse`, 
`uuid.FromString`


[UUID]: https://github.com/gofrs/uuid
[KSUID]: https://github.com/segmentio/ksuid
[cuid]: https://github.com/lucsky/cuid
[Snowflake]: https://github.com/bwmarrin/snowflake
[Sonyflake]: https://github.com/sony/sonyflake
[Sandflake]: https://github.com/celrenheit/sandflake
[ULID]: https://github.com/oklog/ulid
[xid]: https://github.com/rs/xid