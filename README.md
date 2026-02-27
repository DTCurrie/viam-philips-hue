# Module viam-philips-hue

A Viam module for controlling Philips Hue smart lights.

## Setup

You need a Philips Hue Bridge and an API username (key).

### Getting a username

Use the CLI tool to register with your bridge:

```bash
# Build the CLI
go build -o bin/huecli cmd/cli/cmd.go

# Register (will auto-discover your bridge)
./bin/huecli -register
```

This will:

1. Auto-discover your Hue bridge on the network
2. Prompt you to press the link button on the bridge
3. Create and display your username

Save the username for your Viam config.

### Manual registration

If you prefer to do it manually:

1. Find your bridge IP (check your router or use the Hue app)
2. Press the link button on your Hue Bridge
3. Within 30 seconds, run: `curl -X POST http://<bridge-ip>/api -d '{"devicetype":"viam#module"}'`
4. The response will contain your username

## hue-discovery

Discovery service that finds all lights connected to your Hue Bridge. The bridge IP will be discovered automatically if not specified.

```json
{
  "username": "your-api-username-here"
}
```

Or with explicit bridge host:

```json
{
  "bridge_host": "192.168.1.100",
  "username": "your-api-username-here"
}
```

## hue-light-brightness

Controls a single Philips Hue light's on/off state and brightness. Implements the switch interface. The bridge IP will be discovered automatically if not specified.

```json
{
  "username": "your-api-username-here",
  "light_id": 1
}
```

Or with explicit bridge host:

```json
{
  "bridge_host": "192.168.1.100",
  "username": "your-api-username-here",
  "light_id": 1
}
```

### Switch Positions

- Position 0: Light off
- Position 1: Light on at last-set brightness (use 2-100 to choose)
- Position 2-100: Light on at that brightness percentage

## hue-light-color

Controls a single RGB color channel on a Philips Hue light that supports color. Implements the switch interface with a 0–255 range per channel. The bridge IP will be discovered automatically if not specified.

```json
{
  "username": "your-api-username-here",
  "light_id": 1,
  "channel": "red"
}
```

`channel` must be `"red"`, `"green"`, or `"blue"`. Use one component per channel.

### Switch Positions

- Position 0–255: Color channel intensity (maps 1:1 to the 0–255 channel value)

## hue-lights-mode

Controls pre-defined lighting modes across one or more lights. The default mode is `"none"`, which restores lights to their state before any mode was activated. When switching to a mode, the current light state is automatically saved so it can be restored when returning to `"none"`.

The bridge IP will be discovered automatically if not specified.

```json
{
  "username": "your-api-username-here",
  "dance": {
    "left": [1, 2],
    "center": [3, 4],
    "right": [5]
  },
  "daylight": [1, 2, 3],
  "warm": [1, 2, 3]
}
```

Or with explicit bridge host:

```json
{
  "bridge_host": "192.168.1.100",
  "username": "your-api-username-here",
  "dance": {
    "left": [1, 2],
    "center": [3, 4],
    "right": [5]
  },
  "daylight": [1, 2, 3],
  "warm": [1, 2, 3]
}
```

Each mode key takes an array of Hue light IDs to control when that mode is active. Only the modes you want to use need to be configured.

### Switch Positions

- Position 0 (`"none"`): Restore all lights to their saved pre-mode state
- Position 1 (`"dance"`): Staggered color-loop across light groups
- Position 2 (`"daylight"`): Cool daylight white (~6500 K) at full brightness
- Position 3 (`"warm"`): Warm incandescent white (~2700 K) at moderate brightness

### Supported Modes

| Mode       | Effect                                                                                                                                                                                      |
| ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `dance`    | Bridge-native `colorloop` with groups staggered across the hue wheel — lights in a group stay in sync with each other, but different groups cycle through different colors at the same time |
| `daylight` | Sets lights to a crisp daylight white (153 mireds, ~6500 K) at full brightness                                                                                                              |
| `warm`     | Sets lights to a warm incandescent white (370 mireds, ~2700 K) at moderate brightness                                                                                                       |

### Dance mode light groups

The `dance` config takes a **map of group name → light IDs**. All lights in a group are kept in sync with each other. Groups are sorted alphabetically by name and then evenly offset around the full hue wheel (0–65535), so different groups always display different colors.

```json
"dance": {
  "left":   [1, 2],
  "center": [3, 4],
  "right":  [5]
}
```

In this example the three groups are sorted to `center`, `left`, `right`. Group `center` (lights 3 & 4) starts at hue 0, group `left` (lights 1 & 2) starts at hue ~21845, and group `right` (light 5) starts at hue ~43690. All three groups then loop through colors in unison within themselves, but stay a third of the wheel apart from each other at all times.

## CLI Usage

```bash
# Register with the bridge (get a username)
./bin/huecli -register

# List all lights
./bin/huecli -username YOUR_USERNAME

# Control a specific light
./bin/huecli -username YOUR_USERNAME -device "Living Room" -set 50
```
