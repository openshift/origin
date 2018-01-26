package source

import "path/filepath"

// Info is detected platform information from a source directory
type Info struct {
	Platform string
	Version  string
}

// DetectorFunc is a function that returns source Info from a given directory.
// It returns true if it was able to detect the code in the given directory.
type DetectorFunc func(dir string) *Info

// Detectors is a set of DetectorFunc that is used to detect the
// language/platform for a given source directory
type Detectors []DetectorFunc

// DefafultDetectors is a default set of Detector functions
var DefaultDetectors = Detectors{
	DetectRuby,
	DetectJava,
	DetectNodeJS,
	DetectPHP,
	DetectPython,
	DetectPerl,
	DetectScala,
	DetectDotNet,
	DetectLiteralDotNet,
	DetectGolang,
}

// DetectRuby detects Ruby source
func DetectRuby(dir string) *Info {
	return detect("ruby", dir, "Gemfile", "Rakefile", "config.ru")
}

// DetectJava detects Java source
func DetectJava(dir string) *Info {
	return detect("jee", dir, "pom.xml")
}

// DetectNodeJS detects NodeJS source
func DetectNodeJS(dir string) *Info {
	return detect("nodejs", dir, "app.json", "package.json")
}

// DetectPHP detects PHP source
func DetectPHP(dir string) *Info {
	return detect("php", dir, "index.php", "composer.json")
}

// DetectPython detects Python source
func DetectPython(dir string) *Info {
	return detect("python", dir, "requirements.txt", "setup.py")
}

// DetectPerl detects Perl source
func DetectPerl(dir string) *Info {
	return detect("perl", dir, "index.pl", "cpanfile")
}

// DetectScala detects Scala source
func DetectScala(dir string) *Info {
	return detect("scala", dir, "build.sbt")
}

// DetectDotNet detects .NET source and matches it to a dotnet supported annotation or dotnet imagestream name
func DetectDotNet(dir string) *Info {
	return detect("dotnet", dir, "project.json", "*.csproj")
}

// DetectLiteralDotNet detects .NET source and matches it to a .net supported annotation
func DetectLiteralDotNet(dir string) *Info {
	return detect(".net", dir, "project.json", "*.csproj")
}

// DetectGolang detects Go source
func DetectGolang(dir string) *Info {
	return detect("golang", dir, "main.go", "Godeps")
}

// detect returns an Info object with the given platform if the source at dir contains any of the argument files
func detect(platform string, dir string, globs ...string) *Info {
	for _, g := range globs {
		if matches, _ := filepath.Glob(filepath.Join(dir, g)); len(matches) > 0 {
			return &Info{Platform: platform}
		}
	}
	return nil
}
