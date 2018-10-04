// Copyright (c) 2018, Eugene Ponizovsky, <ponizovsky@gmail.com>. All rights
// reserved. Use of this source code is governed by a MIT License that can
// be found in the LICENSE file.

/*
Package conf is an extensible solution for cascading configuration. Package conf
provides configuration processor, that can load configuration layers from
different sources and merges them into the one configuration tree. In addition
configuration processor can expand variables in string values and process _var
and _include directives in resulting configuration tree (see below). Package
conf comes with built-in configuration loaders: fileconf and envconf, and can be
extended by third-party configuration loaders. Package conf do not watch for
configuration changes, but you can implement this feature in the custom
configuration loader. You can find full example in repository.

Configuration processor can expand variables in string values (if you need alias
for complex structures see _var directive). Variable names can be absolute or
relative. Relative variable names begins with "." (dot). The section, in which
a value of relative variable will be searched, determines by number of dots in
variable name. For example, we have a YAML file:
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
 "myapp": map[string]interface{}{
   "mediaFormats": []interface{}{"images", "audio", "video"},

   "dirs": map[string]interface{}{
     "rootDir":      "/myapp",
     "templatesDir": "/myapp/templates",
     "sessionsDir": "/myapp/sessions",

     "mediaDirs": []interface{}{
       "/myapp/media/images",
       "/myapp/media/audio",
       "/myapp/media/video",
     },
   },
 }
To escape variable expansion add one more "$" symbol before variable name.
 templatesDir: "$${myapp.dirs.rootDir}/templates"
After processing we will get:
 templatesDir: "${myapp.dirs.rootDir}/templates"
Package conf supports two special directives in configuration layers: _var and
_include. _var directive assigns configuration parameter value to another
configuration parameter. Argument of the _var directive is a variabale name,
absolute or relative. Here some example:
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
       username: "stat_writer"
       password: "stat_writer_pass"
       options: {_var: "myapp.db.defaultOptions"}

     metrics:
       host: "metrics.mydb.com"
       port: 1234
       dbname: "metrics"
       username: "metrics_writer"
       password: "metrics_writer_pass"
       options: {_var: "...defaultOptions"}

_include directive loads configuration layer from external sources and assigns
it to configuration parameter. Argument of the _include directive is a list of
configuration locators.
 db:
   defaultOptions:
     serverPrepare: true
     expandArray: true
     errorLevel: 2

   connectors: {_include: ["file:connectors.yml"]}
*/
package conf
