# Welcome traveller!

I hope you find this interesting!

## What's the purpose of this project?

It's an attempt at making a Go module and application for enabling
a user to log their drug dosages and timings.
It uses the logged information,
along with fetched and stored locally information about a particular
substance, to make assumptions about where things might be going with
your experience.

The main purpose is harm reduction.

This project does NOT aim the endorsement of any psychoactive substance.
If it prevents a person from consuming more, then it's done it's job.

Showing logs to others as a bragging point makes you look like a fool.

## Examples

If you want to log a dose:

`gopsydose -drug alcohol -route oral -dose 355 -units ml -perc 4.5`

or

`gopsydose -drug weed -route smoked -dose 100 -units mg`

Since both of these aren't consumed at once, there is a command to mark
when the dosing has ended:

`gopsydose -set-end-time`

This will set when you finished your dose for the last log.

If you're consuming something at once like
[LSD](https://en.wikipedia.org/wiki/Lysergic_acid_diethylamide) or
[Psilocybin mushrooms](https://en.wikipedia.org/wiki/Psilocybin_mushroom) or
anything else, there's no need for the
`-set-end-time` command. Just continue without doing it.

To see the last dose: `gopsydose -get-logs-last 1`

To see all dosages: `gopsydose -get-logs`

To see the progress of your last dosage: `gopsydose -get-times`

If you want a log to be remembered and only set the dose for the next log:

`gopsydose -remember -drug weed -route smoked -dose 100 -units mg`

Remembers the config and then for the next dose: `gopsydose -dose 100`

Forgetting the last config: `gopsydose -forget`

If you're running Linux or another UNIX-like OS with GNU Watch,
you can do:

`watch -n 300 gopsydose -get-times`

This will run the command every 5 minutes and show you
the latest results.

There is a limit set in a config file about how many dosages you can do,
the default is 100, when the limit is reached it will not allow anymore,
but you can clean the logs like so: `gopsydose -clean-logs`

No need to delete all logs, you can delete X number of the oldest logs
like so, for example to delete 3 of the oldest logs:

`gopsydose -clean-old-logs 3`

To see where your config files and database file are:

`gopsydose -get-dirs`

Checkout [this](#configs-explained) section for more info on configs!

If you're paranoid, to clean the whole database: `gopsydose -clean-db`

The issue is currently no files are encrypted and can't be 
until a proper implementation is done, also since
by default we're fetching drug information using the psychonautwiki API,
it would be wise not to spam their servers too much.
We store all information locally on first fetch and use only the local info
later for everything. This way even if the Internet goes down, the logger
can still be used properly and the API servers can relax.

For any more info, again: `gopsydose -help`

## Current status

The project is in a very early phase, making the very first steps
at being something remotely usable. If you feel like it, give it a try
and please if you have any ideas, concerns, etc. write in the Issues.

Pull requests are very welcome as well and there currently aren't any
guidelines, just try sticking to the coding style and use `gofmt`!
It would be nice if single lines stay <= 120 characters.

Using this as a module now is a bad idea since the API will most
likely change a lot until the first stable tag.

## Installing Go

Check whether you already have Go, by typing `go version` in your
terminal. If information about Go shows up and you're on Go version 1.17+,
you're good to go. Otherwise remove the old installation and reinstall
with a newer version.

You need to download Go:
* If you're on Windows, you need to download Go
from the official website [here](https://go.dev/).
* If you're on Linux, use your package manager!

[Skip to GCC](#installing-gcc)

### Go Linux Installation

Works on OpenSUSE Tumbleweed. Leap hasn't been tested
and seems like it won't work there.

OpenSUSE: `sudo zypper install go`

Arch Linux: `sudo pacman -S go`

Fedora and Debian might be a bit out of date.

If for Debian the version is old, you need to use backports or sid.
Do this at your own risk, since this is not the normal way of installing
packages.

If for Fedora the version is old, you need to use a package from
a newer release, Rawhide or a third party repository.
The same warning applies as for Debian.

Fedora: `sudo dnf install golang`

Debian: `sudo apt install golang-go`

## Installing GCC

Check whether you already have GCC, by typing `gcc --version` in your
terminal. If information about GCC shows up and you're on GCC version 10+,
you're good to go. Otherwise remove the old installation and reinstall
with a newer version.

You need to download GCC as well:
* On Windows you can download
[tdm-gcc](https://jmeubank.github.io/tdm-gcc/download/).
* On Linux use your package manager!

[Skip to main installation](#installing-this-project)

### GCC Windows installation

The tdm-gcc GUI installer defaults are enough.

### GCC Linux installation

Same info from [Go section](#go-linux-installation)
about Leap, Debian and Fedora versions applies here.

OpenSUSE: `sudo zypper install gcc`

Arch Linux: `sudo pacman -S gcc`

Fedora: `sudo dnf install gcc`

Debian: `sudo apt install gcc-10` or `gcc-11`

## Installing this project

Then you need to run in a terminal:

`go install github.com/psybits/gopsydose@latest`

Afterwards you can do a quick test with: `gopsydose -help`

There are a lot of commands and currently it isn't very clear, but
a few examples and explanations will be left in this document.

## Viewing/Editing the database

If you wish to view/edit the database manually you can
use the [SQLite Browser](https://sqlitebrowser.org/dl/).

Get the database directory using: `gopsydose -get-dirs`

## Configs explained

### gpd-settings.toml

#### MaxLogsPerUser
How many logs a user can make. You can log as a different user using the
`-user` command.

#### UseAPI
The name of the API set in `gpd-sources.toml`. The API needs to have an
implementation, currently only psychonautwiki has one.

#### AutoFetch
Whether to fetch info from an API when logging. If set to false the
source table needs to be manually filled using other tools.

#### DBDir
This is relevant to sqlite.

The directory where the database file will be created. If changing this,
don't forget to check the old directory for any files left!

#### AutoRemove
If set to true, will remove the oldest log when adding a new one, if the
`MaxLogsPerUser` limit is reached, without telling the user.

#### DBDriver
Which database driver to use. Current options are "sqlite3" or "mysql".

#### MySQLAccess
Credentials to access a MySQL database.

Example: user:password@tcp(127.0.0.1:3306)/database

### gpd-sources.toml

```
[NameOfTheApi]
ApiUrl = 'api.whereitis.com'
```

An implementation needs to be present in the code for the name here.

