package def

// Convenience method that calls all forumla validation.
// Modifies the formula.
func ValidateAll(job *Formula) {
	ValidateBasic(job)
	ValidateConvenience(job)
}

// Checks a formula for irrecoverable errors.
// Will NOT modify the formula, with the exception of correcting uninstantiated variables.
func ValidateBasic(job *Formula) {
	// Note, we don't require non-zero outputs, for obvious reasons :)
	if len(job.Inputs) < 1 {
		panic(ValidationError.New("Formula needs at least one input"))
	}

	if job.Inputs[0].Location == "" {
		job.Inputs[0].Location = "/"
	} else if job.Inputs[0].Location != "/" {
		panic(ValidationError.New("First formula input must be mounted to /"))
	}

	if job.Accents.Env == nil {
		job.Accents.Env = map[string]string{}
	}
	if job.Accents.Entrypoint == nil {
		job.Accents.Entrypoint = []string{}
	}
	if job.Accents.Cwd == "" {
		job.Accents.Cwd = "/"
	}
}

// Modifies a formula with a few tweaks that make them more convenient for human-generated input.
// * If no environment PATH was specified, set the PATH to "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
//   * To disable, set job.Accents.Env["PATH"] to any string (including "") as desired.
// * Sets the entrypoint to "/bin/true" if none was specified
//   * To disable, set job.Accents.Entrypoint
func ValidateConvenience(job *Formula) {
	// Add a basic PATH if none exists
	if _, ok := job.Accents.Env["PATH"]; !ok {
		job.Accents.Env["PATH"] = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	}

	// Assume a trivial command if none
	if len(job.Accents.Entrypoint) < 1 {
		job.Accents.Entrypoint = []string{"/bin/true"}
	}
}
