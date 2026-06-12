package quality

// pythonStdlibModules lists top-level standard library modules for Python 3.11+.
// The list intentionally includes a few commonly present older aliases so the
// hallucinated-import check stays conservative on real-world repositories.
var pythonStdlibModules = []string{
	"__future__", "__main__", "_thread", "abc", "aifc", "argparse", "array",
	"ast", "asyncio", "atexit", "audioop", "base64", "bdb", "binascii",
	"bisect", "builtins", "bz2", "calendar", "cgi", "cgitb", "chunk", "cmath",
	"cmd", "code", "codecs", "codeop", "collections", "colorsys", "compileall",
	"concurrent", "configparser", "contextlib", "contextvars", "copy",
	"copyreg", "cProfile", "crypt", "csv", "ctypes", "curses", "dataclasses",
	"datetime", "dbm", "decimal", "difflib", "dis", "doctest", "email",
	"encodings", "ensurepip", "enum", "errno", "faulthandler", "fcntl",
	"filecmp", "fileinput", "fnmatch", "fractions", "ftplib", "functools",
	"gc", "getopt", "getpass", "gettext", "glob", "graphlib", "grp", "gzip",
	"hashlib", "heapq", "hmac", "html", "http", "idlelib", "imaplib",
	"imghdr", "importlib", "inspect", "io", "ipaddress", "itertools", "json",
	"keyword", "lib2to3", "linecache", "locale", "logging", "lzma", "mailbox",
	"mailcap", "marshal", "math", "mimetypes", "mmap", "modulefinder",
	"msilib", "msvcrt", "multiprocessing", "netrc", "nis", "nntplib",
	"ntpath", "numbers", "operator", "optparse", "os", "ossaudiodev",
	"pathlib", "pdb", "pickle", "pickletools", "pipes", "pkgutil", "platform",
	"plistlib", "poplib", "posix", "posixpath", "pprint", "profile", "pstats",
	"pty", "pwd", "py_compile", "pyclbr", "pydoc", "queue", "quopri",
	"random", "re", "readline", "reprlib", "resource", "rlcompleter",
	"runpy", "sched", "secrets", "select", "selectors", "shelve", "shlex",
	"shutil", "signal", "site", "smtplib", "sndhdr", "socket", "socketserver",
	"spwd", "sqlite3", "ssl", "stat", "statistics", "string", "stringprep",
	"struct", "subprocess", "sunau", "symtable", "sys", "sysconfig", "syslog",
	"tabnanny", "tarfile", "telnetlib", "tempfile", "termios", "test",
	"textwrap", "threading", "time", "timeit", "tkinter", "token", "tokenize",
	"tomllib", "trace", "traceback", "tracemalloc", "tty", "turtle",
	"turtledemo", "types", "typing", "unicodedata", "unittest", "urllib",
	"uu", "uuid", "venv", "warnings", "wave", "weakref", "webbrowser",
	"winreg", "winsound", "wsgiref", "xdrlib", "xml", "xmlrpc", "zipapp",
	"zipfile", "zipimport", "zlib", "zoneinfo",
}

var pythonStdlibModuleSet = buildPythonStdlibModuleSet()

func buildPythonStdlibModuleSet() map[string]struct{} {
	out := make(map[string]struct{}, len(pythonStdlibModules))
	for _, name := range pythonStdlibModules {
		out[name] = struct{}{}
	}
	return out
}

// pythonImportAliases maps import names to the PyPI distribution names that
// provide them when the two differ.
var pythonImportAliases = map[string][]string{
	"PIL":           {"pillow"},
	"attr":          {"attrs"},
	"bs4":           {"beautifulsoup4"},
	"cairosvg":      {"cairosvg"},
	"cv2":           {"opencv-python", "opencv-python-headless", "opencv-contrib-python"},
	"dateutil":      {"python-dateutil"},
	"dotenv":        {"python-dotenv"},
	"fitz":          {"pymupdf"},
	"github":        {"pygithub"},
	"google":        {"protobuf", "google-cloud", "google-api-python-client"},
	"jose":          {"python-jose"},
	"jwt":           {"pyjwt"},
	"kafka":         {"kafka-python"},
	"magic":         {"python-magic"},
	"mpl_toolkits":  {"matplotlib"},
	"mysql":         {"mysql-connector-python"},
	"OpenSSL":       {"pyopenssl"},
	"pkg_resources": {"setuptools"},
	"psycopg2":      {"psycopg2", "psycopg2-binary"},
	"serial":        {"pyserial"},
	"setuptools":    {"setuptools"},
	"sklearn":       {"scikit-learn"},
	"slugify":       {"python-slugify"},
	"snowflake":     {"snowflake-connector-python"},
	"telegram":      {"python-telegram-bot"},
	"usb":           {"pyusb"},
	"win32api":      {"pywin32"},
	"win32com":      {"pywin32"},
	"wx":            {"wxpython"},
	"yaml":          {"pyyaml"},
	"zmq":           {"pyzmq"},
}
