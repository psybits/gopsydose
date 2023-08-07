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

## Current status

The project is in a very early phase, making the very first steps
at being something remotely usable. If you feel like it, give it a try
and please if you have any ideas, concerns, etc. write in the Issues.

### Builds

If you don't want to build from source, you can checkout the "Releases"
section. There should be a "snapshot" tag with archives attached to it.

### Contributions

Pull requests are very welcome as well and there currently aren't any
guidelines, just try sticking to the coding style and use `gofmt`!
It would be nice if single lines stay <= 120 characters.

### Module

Using this as a module now is a bad idea, because the API will most
likely change a lot until the first stable tag.

If you're not sure what this means, then you don't need to worry about it.

### Terminal tool changes

If after an update to the terminal tool something broke, it's best to clear
all config files and database files/tables and start over. That's just what
happens when a project is in rapid development without versioning.

If that didn't help with solving the issue, send a report on Github please!

## Source Information

Currently the default source is https://psychonautwiki.org
This means the info present there, that's available through their API, will be
downloaded to your machine and stored in a local database. Later when you log
new dosages or want information about a substance, it will use the local
database, not the API, if it's already fetched the information.

You don't need to configure anything for the database or API by default.

Keep in mind, if you store data locally for too long, it might
be obsolete. You shouldn't clear your local db too quickly as well,
because this way you put more strain on the API servers for no reason and
you might lose data, which you might need in case you don't have an Internet
connection. It's also a lot slower to get data using the API compared to
the local database.

## gpd-names-configs

This directory and it's use might be a bit confusing. This is an attempt at
making a simple explanation.

These files are all written manually, the program does not generate them.

When the program is first ran, it looks for a directory "gpd-names-configs" in
the current working directory. If it finds it, it copies the whole directory
over to the config directory of the operating system user, which depends on
the OS where it's located. If a copy is started, it will print where it wants
to copy to and afterwards the path can be found again by running
`gopsydose -get-paths`.

When initializing the database properly, the contents of all the files in
the now copied over "gpd-names-configs" directory are added to their own tables
in the database. This is done once. The data is not modified, it's only read.
In order to modify the data, the tables must be removed and recreated using the
config files. The gopsydose API contains functions that do this. The CLI tool
has a flag -overwrite-names.

These stored names are used so that different inputs from the users can be
considered valid, even if they're not present in the source.

These names might change when something in the sources change. The config files
can be manually modified to match those changes. The gopsydose source repository
might also reflect those changes and the modifications can be fetched from
there, but should be checked if they're not ahead or behind reality.

After modification, the flag -overwrite-names can be used to reflect the
changes in the database.

### Global alternative names

When adding a new log, the code first checks the global substance
names, which is the "gpd-substance-names.toml" file. Remember, it doesn't check
the file itself, but the data from the file added to the database. It compares
the user input to the "AltNames" in the config file or more accurately,
the "alternativeName" column in the database. If the user input matches one of
the "altenative" names, it replaces the user input with the name set as
"LocalName" in the config file or more accurately the "localName" column
is the database. It then continues to the "source specific names".

### Source specific names

After the code has checked the global names, it checks the
"source-names-local-configs" directory a.k.a. the "source specific names".
The directory contains other directories which correspond to a source.
So the "psychonautwiki" directory has names specific to the
PsychonautWiki source. The config files in this subdirectories have the same
layout as the global configs. They are also stored in the database.
If a match is found, it will replace the replaced "global name" and that's
the final valid output. This is the name used when further processing or
storage is done.

## Terminal tool examples

### Basic options

If you want to log a dose:

`gopsydose -drug alcohol -route drink -dose 500 -units ml -perc 6`

or

`gopsydose -drug weed -route smoked -dose 60 -units mg -perc 5`

or

`gopsydose -drug mdma -route oral -dose 90 -units mg`

Since Cannabis and Alcohol aren't usually consumed at once, there is a command
to mark when the dosing has ended.

`gopsydose -change-log -end-time now`

This will set when you finished your dose for the last log. You can use an
unix timestamp like in the example below instead of `now`.

To change the start time of dose use:
`gopsydose -change-log -start-time 1655443322`

Changing the start time also changes the "id" of a dose.
This means, if you're looking for the last dose and you've changed the start
time to an earlier moment, it will get pushed back in the list.

You can set the times for a specific id by using the `-for-id` command like so:
`gopsydose -change-log -end-time now -for-id 1655443322`

This works for both times.

If you're consuming something at once like
[LSD](https://en.wikipedia.org/wiki/Lysergic_acid_diethylamide) or
[Psilocybin mushrooms](https://en.wikipedia.org/wiki/Psilocybin_mushroom) or
anything else, there's no need for the
`-change-log -end-time` command. Just continue without doing it.

To see the newest dose only: `gopsydose -get-new-logs 1`

To see all dosages: `gopsydose -get-logs`

To search the dosages: `gopsydose -get-logs -search "whatever"`

To see the progress of your last dosage: `gopsydose -get-times`

You can combine `-get-logs` or `-get-times` with: `-for-id`

to get information for a specific ID.

### More options

If you want a log to be remembered and only set the dose for the next log:

`gopsydose -remember -drug weed -route smoked -dose 60 -units mg -perc 5`

Remembers the config and then for the next dose: `gopsydose -dose 60 -perc 5`

Not everything needs `-perc`.

Forgetting the last config: `gopsydose -forget`

If you're running Linux or another UNIX-like OS with GNU Watch,
you can do:

`watch -n 600 gopsydose -get-times`

This will run the command every 10 minutes and show you
the latest results.

Don't run it too fast, because it's bad for your storage device!

There is a limit set in a config file about how many dosages you can do,
the default is 100, when the limit is reached it will not allow anymore,
but you can clean the logs like so: `gopsydose -clean-logs`

To clean dosages with a search: `gopsydose -clean-logs -search "whatever"`

This will clean only dosages which contain the "whatever" string.

No need to delete all logs, you can delete X number of the oldest logs
like so, for example to delete 3 of the oldest logs:

`gopsydose -clean-old-logs 3`

You can do the command like so: `gopsydose -clean-old-logs 1 -for-id 1655144869`

to remove a specific ID, works with `-clean-new-logs 1` as well.

After logging you can change the data of a log using `-change-log` it works for:

`-start-time` ; `-end-time` ; `-drug` ; `-dose` ; `-units` ; `-route`

So for example for changing the dose you would do:

`gopsydose -change-log -dose 123`

This will change the dose for the last log, to change for a specific log do:

`gopsydose -change-log -dose 123 -for-id 1655144869`

To see where your config files and database file are:

`gopsydose -get-paths`

Checkout the [Configs Explained](#configs-explained) section for explanations
on configuration options!

If you're paranoid, to clean the whole database: `gopsydose -clean-db`

Also don't forget, if you're using sqlite, which is the default, you can always
do: `gopsydose -get-paths`

Then you can delete the database (db) file itself manually.

### Even more options

Every single option is described quickly from the -help flag.

## Security/Privacy

The issue is currently no files are encrypted and can't be 
until a proper implementation is done. Also since
by default drug information is fetched using the psychonautwiki API,
it would be wise not to spam their servers too much.
The information is stored locally on first fetch and reused later for
everything. This way even if the Internet goes down, the logger
can still be used properly and the API servers can relax.

For any more info, again: `gopsydose -help`

### Proxy

In order to use a proxy, checkout the [ProxyURL](#proxyurl) setting!

The address "socks5://127.0.0.1:9150" is for connection via Tor, using an
opened Tor browser. When the browser is opened, it also starts a proxy server
with that address, so it can be used to query data using Tor. Using the browser
is the easier method, there's no need to run services/daemons manually and
configuring them.

## Alias

You can use the `example.alias` file to setup an easier to use environment
for your system. The difference between this and the `-remember` command is
that, shell aliases are system wide and from the operating system. The
`-remember` command stores the "command" in the database, allowing even remote
use of those settings, which is OS-independent. Aliases are a lot more flexible
currently, so use them when you need them!

## Installing Go

* When using Windows, Go can be downloaded
from the official website https://go.dev
* When using Linux, a package manager can be used.
Checkout the [Go Linux Installation](#go-linux-installation) section!

### Go Linux Installation

For OpenSUSE, Tumbleweed is preferable, since Leap would probably be falling
behind a bit with the versions.

OpenSUSE: `sudo zypper install go`

Arch Linux: `sudo pacman -S go`

Fedora and Debian might be a bit out of date as well.

If for Debian the version is old, backports or sid need to be used.
Do this at your own risk, since this is not the normal way of installing
packages.

If for Fedora the version is old, a package from a newer release, Rawhide or
a third party repository needs to be used.
The same warning applies as for Debian.

Fedora: `sudo dnf install golang`

Debian: `sudo apt install golang-go`

### Using asdf

If the distribution package manager isn't used, there's also an option
with an external tool. Checkout https://asdf-vm.com

## Installing this project

To install the gopsydose terminal tool, run in terminal:

`go install github.com/psybits/gopsydose@latest`

Afterwards a quick test can be done in the terminal: `gopsydose -help`

It should show a list of commands and their descriptions. If not, something
went wrong in the installation.

There are a lot of commands and currently it isn't very clear, but
a few examples and explanations are present in this document.

## Viewing/Editing the database

To view or edit the sqlite database manually,
[SQLite Browser](https://sqlitebrowser.org/dl/) can be used.

For MariaDB/MySQL, [DBeaver](https://dbeaver.io/) can be used.

To get the database directory, run in terminal: `gopsydose -get-paths`

## Configs explained

### gpd-settings.toml

#### MaxLogsPerUser
How many logs a user can make. You can log as a different user using the
`-user` command.

#### UseSource
The name of the API set in `gpd-sources.toml`. The API needs to have an
implementation, currently only psychonautwiki has one.

#### AutoFetch
Whether to fetch info from an API when logging. If set to false the
source table needs to be manually filled using other tools.

#### AutoRemove
If set to true, will remove the oldest log when adding a new one, if the
`MaxLogsPerUser` limit is reached, without telling the user.

#### DBDriver
Which database driver to use. Current options are "sqlite" or "mysql".

#### VerbosePrinting
If set to true, functions which print more verbose information, will print it.

#### Timezone
This by default is "Local", which means the code tries to figure out the local
time zone using the operating system settings. If this fails, you can use
the other possible strings from below.

You can change it to something like "Europe/Paris" to change the time zone to
the one in that area.

All info about this string can be found here:
https://pkg.go.dev/time#LoadLocation

#### ProxyURL
This by default is '' (empty), meaning no proxy is used. It can also be set to
'none' with the same effect. If the string is set to 'socks5://127.0.0.1:9150',
all query connections are made using that proxy. This specific URL is the
default for the Tor browser, so it can be used to query via the Tor network if
the browser is open while querying.

#### Timeout
This setting depends on the way the API is used. Some developers might choose
to ignore it, some might want to use it. When used, it's supposed to set the
time to wait for a SQL query to finish or a connection to a server to be done.
Some developers might use it as the total duration for all connections or
queries, for example in the case of terminal programs.

When ignored, it would be good for the developers to set the value to something
like "This Value Is Ignored".

If your connections have huge delays for some reason and this settings is
not ignored, try setting the value higher.

The possible values are described here:
https://pkg.go.dev/time#ParseDuration

#### CostCurrency
Sets a default currency to log when using the `-cost` flag. It can be bypassed
using the `-cost-cur` flag per log.

#### DBSettings

##### DBSettings.mysql

Path - Credentials to access a MariaDB/MySQL database.

The database needs to be created in advance.

Example: user:password@tcp(127.0.0.1:3306)/database

##### DBSettings.sqlite

Path - Disk location to access the sqlite db file.

The database file will be automatically created, no need
to do it in advance.

Example: /home/user/.local/share/GPD/gpd.db

### gpd-sources.toml

```
[NameOfTheApi]
API_ADDRESS = 'api.whereitis.com'
```

An implementation needs to be present in the code for the name of the API.

