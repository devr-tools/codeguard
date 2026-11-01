package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func pythonTaintReport(t *testing.T, name string, files map[string][]string) codeguard.Report {
	t.Helper()
	dir := t.TempDir()
	for file, lines := range files {
		writeFile(t, filepath.Join(dir, file), strings.Join(lines, "\n"))
	}
	report, err := codeguard.Run(context.Background(), securityOnlyConfig(name, dir, "python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return report
}

// Flask request data reached through the qualified flask.request module
// attribute is the same untrusted input as the bare imported request.
func TestPythonTaintQualifiedFlaskRequestToShellAndSSRF(t *testing.T) {
	report := pythonTaintReport(t, "taint-py-flask-qualified", map[string][]string{"app.py": {
		"import os",                   // 1
		"import subprocess",           // 2
		"import requests",             // 3
		"import flask",                // 4
		"",                            // 5
		"app = flask.Flask(__name__)", // 6
		"",                            // 7
		"@app.route('/run')",          // 8
		"def run():",                  // 9
		"    cmd = flask.request.args.get('cmd')", // 10
		"    subprocess.run(cmd, shell=True)",     // 11 taint
		"    os.system(flask.request.form['x'])",  // 12 taint
		"    payload = flask.request.get_json()",  // 13
		"    os.popen(payload['c'])",              // 14 taint
		"    target = flask.request.args['url']",  // 15
		"    requests.post(target, data={})",      // 16 ssrf
		"    return 'ok'",                         // 17
		"",
	}})
	assertTaintLines(t, report, "security.taint.python", 11, 12, 14)
	assertTaintLines(t, report, "security.ssrf.python", 16)
	assertChainMessage(t, taintMessages(t, report, "security.taint.python"), "flask.request.args", "subprocess.run", "cmd")
}

// Sinks imported by name or under an alias resolve to the canonical module
// function, so `from urllib.request import urlopen` is still an SSRF sink.
func TestPythonTaintImportedSinkAliasesResolve(t *testing.T) {
	report := pythonTaintReport(t, "taint-py-import-alias", map[string][]string{"views.py": {
		"import subprocess as sp",                         // 1
		"from os import system, popen",                    // 2
		"from subprocess import check_output",             // 3
		"from urllib.request import urlopen",              // 4
		"from requests import get as fetch",               // 5
		"from flask import request",                       // 6
		"",                                                // 7
		"def handler():",                                  // 8
		"    sp.run(request.args['a'], shell=True)",       // 9 taint
		"    system(request.form['b'])",                   // 10 taint
		"    popen(request.values['c'])",                  // 11 taint
		"    check_output(request.json['d'], shell=True)", // 12 taint
		"    urlopen(request.args.get('u'))",              // 13 ssrf
		"    fetch(request.args['v'])",                    // 14 ssrf
		"    return 'ok'",                                 // 15
		"",
	}})
	assertTaintLines(t, report, "security.taint.python", 9, 10, 11, 12)
	assertTaintLines(t, report, "security.ssrf.python", 13, 14)
	assertChainMessage(t, taintMessages(t, report, "security.ssrf.python"), "request.args", "urllib.request.urlopen")
}

// Django class-based and function views read GET/POST from the request
// parameter; both reach shell and outbound-HTTP sinks.
func TestPythonTaintDjangoRequestToShellAndSSRF(t *testing.T) {
	report := pythonTaintReport(t, "taint-py-django-shell", map[string][]string{"views.py": {
		"import os",                            // 1
		"import subprocess",                    // 2
		"import requests",                      // 3
		"from django.http import HttpResponse", // 4
		"",                                     // 5
		"def view(request):",                   // 6
		"    subprocess.call(request.GET['c'], shell=True)",      // 7 taint
		"    os.system(request.POST.get('d'))",                   // 8 taint
		"    requests.get(request.GET['u'])",                     // 9 ssrf
		"    return HttpResponse('ok')",                          // 10
		"",                                                       // 11
		"class Report:",                                          // 12
		"    def get(self, request):",                            // 13
		"        name = request.GET.get('name')",                 // 14
		"        subprocess.Popen('report ' + name, shell=True)", // 15 taint
		"        return HttpResponse('ok')",                      // 16
		"",
	}})
	assertTaintLines(t, report, "security.taint.python", 7, 8, 15)
	assertTaintLines(t, report, "security.ssrf.python", 9)
}

// True negatives: sanitized values, constant commands, shell=False with an
// argv list, parameterized SQL, and request data confined to non-URL
// arguments of an HTTP call.
func TestPythonTaintRequestTrueNegatives(t *testing.T) {
	report := pythonTaintReport(t, "taint-py-negatives", map[string][]string{"app.py": {
		"import os",
		"import shlex",
		"import subprocess",
		"import requests",
		"from urllib.request import urlopen",
		"from flask import request",
		"",
		"def handler(cursor):",
		"    subprocess.run(shlex.quote(request.args['x']), shell=True)",
		"    subprocess.run(['ls', request.args['d']], shell=False)",
		"    subprocess.run(['ls', request.args['d']])",
		"    os.system('ls -la')",
		"    n = int(request.args['n'])",
		"    os.system('sleep %d' % n)",
		"    cursor.execute('SELECT * FROM t WHERE id = %s', (request.args['id'],))",
		"    requests.get('https://api.service.test/search', params={'q': request.args['q']})",
		"    urlopen('https://api.service.test/health')",
		"    return 'ok'",
		"",
	}})
	assertTaintLines(t, report, "security.taint.python")
	assertTaintLines(t, report, "security.ssrf.python")
}

// A local object that merely happens to be named request, in a module that
// imports no web framework, is not a request source.
func TestPythonTaintRequestSourceStaysFrameworkGated(t *testing.T) {
	report := pythonTaintReport(t, "taint-py-request-gated", map[string][]string{"tool.py": {
		"import subprocess",
		"from collections import namedtuple",
		"",
		"Request = namedtuple('Request', 'args')",
		"request = Request(args={'cmd': 'ls'})",
		"",
		"def main():",
		"    subprocess.run(request.args['cmd'], shell=True)",
		"",
	}})
	assertTaintLines(t, report, "security.taint.python")
}

// httpx.request/stream take the method first, so only the URL argument is a
// sink; a tainted method name alone is not SSRF.
func TestPythonTaintHTTPXMethodFirstSinks(t *testing.T) {
	report := pythonTaintReport(t, "taint-py-httpx", map[string][]string{"app.py": {
		"import httpx",                     // 1
		"from flask import request",        // 2
		"",                                 // 3
		"def proxy():",                     // 4
		"    target = request.args['url']", // 5
		"    httpx.request('GET', target)", // 6 ssrf
		"    httpx.put(target, json={})",   // 7 ssrf
		"    httpx.request(request.args['m'], 'https://api.service.test/')", // 8 constant URL
		"    return 'ok'", // 9
		"",
	}})
	assertTaintLines(t, report, "security.ssrf.python", 6, 7)
}
