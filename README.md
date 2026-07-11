# Das wetter

Das wetter is a fast weather CLI for humans and scripts. `weather` is one-shot
and pipeline-friendly; `das wetter` opens an interactive terminal prompt. It
uses Open-Meteo by default, needs no API key, and writes clean terminal output
or structured JSON.

```text
› ~ das wetter

Atlanta, GA
Saturday, July 11, 2026 4:45 PM · Open-Meteo

  🌤 82°F                         │ 💨 Wind: 8 mph
    Partly Cloudy                 │ 💧 Humidity: 65%
    Feels like 86°F               │ ☁ Cloud cover: 30%
                                  │ 🌅 Sunset: 8:51 PM
────────────────────────────────────────────────────────────

Forecast
  Sat  🌤   87°F   68°F  Sunny
  Sun  🌧   84°F   71°F  Showers

Tip: weather --json · weather --hours 6 · weather tomorrow · weather help
```

When attached to a terminal, the human-readable view adds restrained ANSI
color for hierarchy. Colors automatically turn off for pipes, `TERM=dumb`, or
when `NO_COLOR` is set.

## Install

Run the installer from any directory:

```sh
"/path/to/Das Wetter/install.sh"
```

It builds an optimized, dependency-free Go binary when Go is available. If Go
is not installed, it uses the repository's prebuilt `weather` binary. The
primary command is installed at `~/.local/bin/weather`.

If `~/.local/bin` is not already on your `PATH`, add this to `~/.zshrc` and
restart the terminal:

```sh
export PATH="$HOME/.local/bin:$PATH"
```

Then run:

```sh
weather
# or launch the interactive prompt
das wetter
```

The installer also provides `das-wetter` and `das wetter` aliases.

## Build from source

Go is required only for development or building from source:

```sh
go build -o weather .
./weather "Atlanta, GA"
```

For a smaller static binary:

```sh
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -buildid=" -o weather .
```

## Install with Homebrew

Homebrew installs third-party apps through a tap. The included
`Formula/das-wetter.rb.template` is ready for a personal tap once this project
has a GitHub repository and a tagged release.

1. Push the source repository, for example
   `https://github.com/YOUR_USER/das-wetter`, then create a release tag:

   ```sh
   git tag v0.1.0
   git push origin v0.1.0
   ```

2. Create a second public repository named `homebrew-das-wetter`. In that
   repository, copy the template into `Formula/das-wetter.rb`, replace
   `REPLACE_OWNER`, and fill in the release archive checksum:

   ```sh
   mkdir -p Formula
   cp ../Das\ Wetter/Formula/das-wetter.rb.template Formula/das-wetter.rb
   curl -L https://github.com/YOUR_USER/das-wetter/archive/refs/tags/v0.1.0.tar.gz \
     | shasum -a 256
   ```

   Commit and push the formula to the tap repository.

3. Install and launch it:

   ```sh
   brew tap YOUR_USER/das-wetter
   brew install YOUR_USER/das-wetter/das-wetter
   das wetter
   ```

   The formula provides `weather`, `das-wetter`, and `das` aliases. For later
   releases, push a new version tag, update the formula URL and checksum, then
   run `brew update && brew upgrade das-wetter`.

## Usage

```text
weather [location ...] [flags]
```

```sh
weather atl
weather "Atlanta, GA"
weather atl nyc la
```

Quote locations containing spaces. Separate unquoted arguments are treated as
separate locations and shown as a compact comparison.

With no location, `weather` resolves the current approximate public-IP
location first. If geolocation or its forecast request is unavailable, it
falls back to the last successfully cached location and then
`default_location` from `~/.weatherconfig`. The IP address itself is neither
printed nor stored.

### Flags

| Flag | Description |
| --- | --- |
| `--json` | Print structured JSON only. |
| `--tomorrow` | Show tomorrow's forecast. |
| `--hours N` | Include the next `N` hourly forecasts. |
| `--alerts` | Include active NOAA alerts for US locations. |
| `--watch N` | Refresh every `N` seconds until Ctrl-C. |
| `--units imperial\|metric` | Override the configured units. |
| `--compare` | Use compact output for multiple locations. |
| `--graph` | Render a 12-hour ASCII temperature graph. |
| `--if "EXPRESSION"` | Evaluate a condition and set the exit status. |
| `--astro` | Show moon phase and estimated viewing quality. |

Examples:

```sh
weather "Atlanta, GA" --tomorrow
weather London --units metric --hours 6
weather atl nyc la --compare
weather atl --graph
weather atl --alerts
weather atl --astro
weather atl --watch 30
weather atl --json
```

When launched as `das wetter` in a real terminal, the app stays open at a `~`
prompt. Type `--json`, `--hours 6`, `tomorrow`, `help`, `refresh`, or `quit`.
Piped and scripted invocations remain one-shot.

The common command-word shortcuts also work:

```sh
weather help
weather refresh
weather hours 6
weather tomorrow
weather metric
weather quit
```

JSON output is compact, contains no ANSI styling, and is suitable for `jq` or
another program:

```sh
weather atl --json | jq '.temperature'
```

## Conditions and exit codes

Conditions support `>`, `<`, `>=`, `<=`, `==`, and `!=` against `rain`,
`temp`, `feels`, `humidity`, `wind`, or `cloud` (their longer field names also
work).

```sh
weather atl --if "rain > 50"
weather atl --if "temp < 32" --json >/dev/null
```

Exit codes:

- `0`: request succeeded, or the condition is true
- `1`: the `--if` condition is false
- `2`: invalid usage, location failure, or no live/cached weather is available

## Configuration and cache

Optional configuration lives at `~/.weatherconfig`:

```ini
default_location=atl
units=imperial
```

Successful responses are cached in `~/.weather/cache.json` for 10 minutes.
Every normal request tries the live API first. If that request fails, a recent
matching cache entry is used and clearly labeled as cached; entries older than
10 minutes are not presented as current weather.

Human output includes the provider and the data-valid timestamp. JSON includes
`source`, `valid_at`, and `fetched_at`, making it possible to verify exactly
where and when each response came from.

## Data sources

- **[Open-Meteo](https://open-meteo.com/en/docs)**: geocoding and weather forecasts; no key required
- **[NOAA Weather.gov](https://www.weather.gov/documentation/services-web-api)**: active severe-weather alerts for US coordinates
- **[WeatherAPI.com](https://www.weatherapi.com/docs/)**: optional fallback when `WEATHERAPI_KEY` is set
- **ipwho.is**, then **ipapi.co**: approximate location when no location is known

To enable the optional forecast fallback:

```sh
export WEATHERAPI_KEY="your-key"
weather atl
```

No runtime framework or service is installed. The CLI is a single binary and
embeds timezone data; it uses only outbound HTTPS requests plus its local
config and cache files (and the host operating system's standard libraries).

## Development

```sh
go test ./...
go vet ./...
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -buildid=" -o weather .
```
