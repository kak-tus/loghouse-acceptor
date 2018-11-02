// Copyright (c) 2018, Eugene Ponizovsky, <ponizovsky@gmail.com>. All rights
// reserved. Use of this source code is governed by a MIT License that can
// be found in the LICENSE file.

/*
Package conf is an extensible solution for cascading configuration. Package conf
provides configuration processor, that can load configuration layers from
different sources and merges them into the one configuration tree. In addition
configuration processor can expand references on configuration parameters in
string values, and process _ref and _include directives in resulting configuration
tree (see below). Package conf comes with built-in configuration loaders: fileconf
and envconf, and can be extended by third-party configuration loaders. Package
conf do not watch for configuration changes, but you can implement this feature
in the custom configuration loader. You can find full example in repository.

Configuration processor can expand references on configuration parameters in
string values (if you need reference on complex structures see _ref directive).
Reference names can be absolute or relative. Relative reference names begins
with "." (dot). The section, in which a value of relative reference will be
searched, determines by number of dots in reference name. For example, we have
a YAML file:

 myapp:
   mediaFormats: ["images", "audio", "video"]

   dirs:
     rootDir: "/myapp"
     templatesDir: "${myapp.dirs.rootDir}/templates"
     sessionsDir: "${.rootDir}/sessions"

     mediaDirs:
       - "${..rootDir}/media/${myapp.mediaFormats.0}"
       - "${..rootDir}/media/${myapp.mediaFormats.1}"
       - "${..rootDir}/media/${myapp.mediaFormats.2}"

After processing of the file we will get a map:

 "myapp": conf.M{
   "mediaFormats": conf.S{"images", "audio", "video"},

   "dirs": conf.M{
     "rootDir": "/myapp",
     "templatesDir": "/myapp/templates",
     "sessionsDir": "/myapp/sessions",

     "mediaDirs": conf.S{
       "/myapp/media/images",
       "/myapp/media/audio",
       "/myapp/media/video",
     },
   },
 }

To escape expansion of reference, add one more "$" symbol before reference name.

 templatesDir: "$${myapp.dirs.rootDir}/templates"

After processing we will get:

 templatesDir: "${myapp.dirs.rootDir}/templates"

Package conf supports special directives in configuration layers: _ref and
_include. _ref directive retrieves a value by reference on configuration parameter
and assigns this value to another configuration parameter. _ref directive can
take three forms:

 _ref: <name>
 _ref: {name: <name>, default: <value>}
 _ref: {firstDefined: [<name1>, ...], default: <value>}

In the first form _ref directive just assings a value retrieved by reference.
In the second form _ref directive tries to retrieve a value by reference and, if
no value retrieved, assigns default value. And in the third form _ref directive
tries to retrive a value from the first defined reference and, if no value
retrieved, assigns default value. Default value in second and third forms can be
omitted. Reference names in _ref directive can be relative or absolute.

 db:
   defaultOptions:
     serverPrepare: true
     expandArray: true
     errorLevel: 2

   connectors:
     stat:
       host: "stat.mydb.com"
       port: 1234
       dbname: "stat"
       username: "stat"
       password:
         _ref: {name: "MYAPP_DB_STAT_PASSWORD", default: "stat_pass"}
       options: {_ref: "db.defaultOptions"}

     metrics:
       host: "metrics.mydb.com"
       port: 1234
       dbname: "metrics"
       username: "metrics"
       password:
         _ref: {name: "MYAPP_DB_METRICS_PASSWORD", default: "metrics_pass"}
       options: {_ref: "...defaultOptions"}

_include directive loads configuration layer from external sources and inserts
it to configuration tree. _include directive accepts as argument a list of
configuration locators.

 db:
   defaultOptions:
     serverPrepare: true
     expandArray: true
     errorLevel: 2

   connectors: {_include: ["file:connectors.yml"]}
*/
package conf
