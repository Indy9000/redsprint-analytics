# Analytics Analyser

A Dart CLI tool to query and analyze JSON analytics events stored on disk.

## Installation

```bash
cd analyser
dart pub get
```

## Usage

From the `analyser` directory:

```bash
dart run bin/analyser.dart [options] <command> [arguments]
```

### Options

- `-d, --data <path>` - Path to data directory (default: `../bin/data`, supports both relative and absolute paths)
- `-a, --app <app_id>` - Filter by app ID
- `-s, --start <YYYYMMDD>` - Start date filter
- `-e, --end <YYYYMMDD>` - End date filter
- `-f, --filter <expression>` - Filter expression (e.g., `field=value`, `field!=value`, `field contains value`)
- `-h, --help` - Show help

## Commands

### List Commands

#### `apps`
List all app IDs in the data directory.

```bash
dart run bin/analyser.dart apps
```

#### `dates`
Show date ranges for each app.

```bash
dart run bin/analyser.dart dates
```

### Analysis Commands

#### `summary`
Show comprehensive analytics summary including total events, unique users, sessions, and breakdowns by app, event type, platform, and locale.

```bash
# All apps
dart run bin/analyser.dart -d ../bin/data summary

# Specific app
dart run bin/analyser.dart -d ../bin/data -a 1810d3dc summary

# Date range
dart run bin/analyser.dart -d ../bin/data -s 20250810 -e 20250820 summary
```

#### `count <field>`
Count events grouped by a specific field.

```bash
# Count by event type
dart run bin/analyser.dart -d ../bin/data count event_type

# Count by locale
dart run bin/analyser.dart -d ../bin/data count device.locale

# Count page views by URL
dart run bin/analyser.dart -d ../bin/data count properties.page_url
```

#### `top <field> <n>`
Show top N values for a field.

```bash
# Top 10 pages
dart run bin/analyser.dart -d ../bin/data top properties.page_url 10

# Top 5 locales
dart run bin/analyser.dart -d ../bin/data top device.locale 5

# Top referrers
dart run bin/analyser.dart -d ../bin/data top web_specific.referrer 10
```

#### `unique <field>`
Count unique values for a field.

```bash
# Unique users
dart run bin/analyser.dart -d ../bin/data unique user.anonymous_id

# Unique sessions
dart run bin/analyser.dart -d ../bin/data unique user.session_id

# Unique IPs
dart run bin/analyser.dart -d ../bin/data unique location.ip
```

#### `filter <field> <operator> <value>`
Filter events by field value.

Operators: `=`, `!=`, `>`, `<`, `>=`, `<=`, `contains`

```bash
# Only click events
dart run bin/analyser.dart -d ../bin/data filter event_type = click

# Events from specific locale
dart run bin/analyser.dart -d ../bin/data filter device.locale = en-GB

# Page URLs containing 'tools'
dart run bin/analyser.dart -d ../bin/data filter properties.page_url contains tools
```

#### `group <field> <aggregate_field> <function>`
Group by field and apply aggregate function.

Functions: `count`, `sum`, `avg`, `min`, `max`

```bash
# Average load time by locale
dart run bin/analyser.dart -d ../bin/data group device.locale properties.load_time avg

# Total events by event type
dart run bin/analyser.dart -d ../bin/data group event_type event_id count
```

### Time-based Analysis

#### `by-date`
Show events grouped by date.

```bash
dart run bin/analyser.dart -d ../bin/data by-date
```

#### `by-hour`
Show events distribution by hour of day with visual bars.

```bash
dart run bin/analyser.dart -d ../bin/data by-hour
```

## Field Reference

### Top-level Fields
- `event_id` - Event UUID
- `timestamp` - Event timestamp (DateTime)
- `app_id` - Application ID
- `event_type` - Type of event (e.g., page_view, click)
- `event_name` - Specific event name

### User Fields
- `user.session_id` - Session identifier
- `user.anonymous_id` - Anonymous user identifier

### Device Fields
- `device.platform` - Platform (e.g., web)
- `device.os_version` - Operating system version
- `device.device_model` - Device model
- `device.screen_resolution` - Screen resolution
- `device.locale` - User locale
- `device.timezone` - User timezone

### Location Fields
- `location.ip` - IP address

### Web-Specific Fields
- `web_specific.user_agent` - Browser user agent
- `web_specific.referrer` - Referrer URL
- `web_specific.page_url` - Current page URL
- `web_specific.page_title` - Page title
- `web_specific.utm_source` - UTM source parameter
- `web_specific.utm_medium` - UTM medium parameter
- `web_specific.utm_campaign` - UTM campaign parameter
- `web_specific.utm_content` - UTM content parameter
- `web_specific.utm_term` - UTM term parameter
- `web_specific.utm_id` - UTM ID parameter

### Properties (varies by event)
- `properties.page_url` - Page URL
- `properties.page_title` - Page title
- `properties.load_time` - Page load time
- `properties.dns_time` - DNS lookup time
- `properties.tcp_time` - TCP connection time
- `properties.ttfb` - Time to first byte
- `properties.dom_ready` - DOM ready time
- `properties.tag` - HTML tag (for clicks)
- `properties.text` - Element text (for clicks)
- `properties.href` - Link href (for clicks)
- `properties.x`, `properties.y` - Click coordinates

## Example Queries

### General Analytics

```bash
# Get overall analytics summary
dart run bin/analyser.dart -d ../bin/data summary

# Find most popular pages
dart run bin/analyser.dart -d ../bin/data top properties.page_url 10

# Count clicks by element
dart run bin/analyser.dart -d ../bin/data -a 1810d3dc filter event_type = click

# Analyze traffic by hour
dart run bin/analyser.dart -d ../bin/data by-hour

# Get unique daily active users
dart run bin/analyser.dart -d ../bin/data -s 20250810 -e 20250810 unique user.anonymous_id

# Average page load time by device
dart run bin/analyser.dart -d ../bin/data group device.device_model properties.load_time avg

# Events from mobile devices
dart run bin/analyser.dart -d ../bin/data filter device.screen_resolution contains x739
```

### Campaign Analysis

Analyze the effectiveness of marketing campaigns using UTM parameters and the `--filter` option:

```bash
# List all campaigns
dart run bin/analyser.dart -d ../bin/data count web_specific.utm_campaign

# Top traffic sources
dart run bin/analyser.dart -d ../bin/data top web_specific.utm_source 10

# Count events by medium (paid, organic, social, etc.)
dart run bin/analyser.dart -d ../bin/data count web_specific.utm_medium

# Top actions (event_name) from a specific campaign
dart run bin/analyser.dart -d ../bin/data --filter "web_specific.utm_campaign=120241818854010670" count event_name

# Get unique users from Facebook campaigns
dart run bin/analyser.dart -d ../bin/data --filter "web_specific.utm_source=fb" unique user.anonymous_id

# Events by hour for a specific campaign
dart run bin/analyser.dart -d ../bin/data --filter "web_specific.utm_campaign=120241818854010670" by-hour

# Summary for paid traffic only
dart run bin/analyser.dart -d ../bin/data --filter "web_specific.utm_medium=paid" summary

# Click events from a specific campaign
dart run bin/analyser.dart -d ../bin/data --filter "web_specific.utm_campaign=120241818854010670" --filter "event_type=click" count properties.text

# Top pages from Facebook traffic
dart run bin/analyser.dart -d ../bin/data --filter "web_specific.utm_source=fb" top properties.page_url 10

# Unique sessions per campaign
dart run bin/analyser.dart -d ../bin/data count web_specific.utm_campaign
```

## Advanced Usage

### Chaining with Shell Commands

You can combine the analyser with standard shell tools for more complex analysis:

```bash
# Export to CSV
dart run bin/analyser.dart count event_type > event_counts.txt

# Filter and count
dart run bin/analyser.dart -a 1810d3dc summary | grep "Unique"
```

### Multiple Filters

For complex queries requiring multiple filters, you can run sequential commands or modify the code to support multiple filter arguments.

## Tips

1. Use the `-a` flag to focus on a specific app for faster queries
2. Date filters (`-s`, `-e`) can significantly speed up analysis for large datasets
3. The `summary` command gives a quick overview before diving into specific queries
4. Use `unique` to measure actual user engagement vs raw event counts
5. The `by-hour` command helps identify peak usage times
