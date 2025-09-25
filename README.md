# Touch-Block
An application created to prevent touch input on my wall mounted home assistant kiosks when the display is asleep. This became an issue as the displays I use take approx 2 seconds to wake up and during that time the user could be tapping on switches without realising.

## Usage
By default, this application will poll the DPMS extension in x11 every 50ms to check if the display is awake or asleep, on a transition it will then show a full screen X11 window overlay that will prevent all touch input (right now this also captures any form of cursor input rather than just touch).

`--dpms=false` Switches to using xrandr as the source for testing whether the display is on or off, although it's a mess and I have no idea if it even works correctly...

`--delay=<TIME_IN_MS>` Adds an additional delay in milliseconds before removing the touch blocking, tweak this to your display, each display has a different amount of time it takes to wake up.

`--dpms-poll=<TIME_IN_MS>` Changes the interval in milliseconds between each check of DPMS to test whether the display is on or off, has no effect if xrandr is being used. 

```
touch-block --dpms=true --delay=500 --dpms-poll=50
```