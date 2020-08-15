def bashTarget(name, script, env):
    return target(
        name=name,
        builder="bash",
        args=["-c", script],
        env=env,
    )
