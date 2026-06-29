#!/usr/bin/env node
// set-android-version.mjs
//
// Injects the Semantic_Version into the Capacitor-generated Android project so
// the Android_App carries a MAJOR.MINOR.PATCH identifier (Requirement 15.9).
//
// Capacitor's Android build defines the user-facing version in
//   android/app/build.gradle
// via `versionName "x.y.z"` and an integer `versionCode`. That native project
// is produced by `npx cap add android` and is NOT committed in this repo yet
// (it requires the Android SDK/Java; see tasks 22.2 / 22.4). This script is the
// mechanism the release workflow (task 22.4) calls to stamp the version from
// the pushed git tag before building the APK/AAB:
//
//   node scripts/set-android-version.mjs 1.4.2
//   # or, falling back to the npm package version / VOLT_VERSION env:
//   npm run android:set-version
//
// When android/app/build.gradle does not exist (the native project has not been
// generated in this environment), the script is a no-op that prints a warning
// and exits 0, so it never breaks a build that legitimately has no Android
// project present.

import { readFileSync, writeFileSync, existsSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'

const __dirname = dirname(fileURLToPath(import.meta.url))
const frontendDir = resolve(__dirname, '..')
const gradlePath = resolve(frontendDir, 'android', 'app', 'build.gradle')

/** Resolve the semantic version from argv, then VOLT_VERSION, then package.json. */
function resolveVersion() {
  const argVersion = process.argv[2]
  if (argVersion && argVersion.trim() !== '') return argVersion.trim().replace(/^v/, '')

  const envVersion = process.env.VOLT_VERSION
  if (envVersion && envVersion.trim() !== '') return envVersion.trim().replace(/^v/, '')

  try {
    const pkg = JSON.parse(readFileSync(resolve(frontendDir, 'package.json'), 'utf8'))
    if (pkg.version && pkg.version !== '0.0.0') return String(pkg.version)
  } catch {
    // ignore – fall through to error below
  }
  return null
}

const SEMVER = /^\d+\.\d+\.\d+$/

const version = resolveVersion()
if (version == null) {
  console.error(
    'set-android-version: no version provided. Pass one as an argument, set VOLT_VERSION, ' +
      'or give package.json a non-default "version".',
  )
  process.exit(1)
}

if (!SEMVER.test(version)) {
  console.error(`set-android-version: "${version}" is not a MAJOR.MINOR.PATCH semantic version.`)
  process.exit(1)
}

// versionCode must be a monotonically increasing integer. Derive a stable code
// from the semver parts: MAJOR*10000 + MINOR*100 + PATCH (each part assumed < 100).
const [major, minor, patch] = version.split('.').map(Number)
const versionCode = major * 10000 + minor * 100 + patch

if (!existsSync(gradlePath)) {
  console.warn(
    `set-android-version: ${gradlePath} not found. The Android native project has not been ` +
      'generated yet (run `npx cap add android` in a provisioned environment). Skipping ' +
      `version injection (would have set versionName "${version}", versionCode ${versionCode}).`,
  )
  process.exit(0)
}

let gradle = readFileSync(gradlePath, 'utf8')

const hadVersionName = /versionName\s+"[^"]*"/.test(gradle)
const hadVersionCode = /versionCode\s+\d+/.test(gradle)

gradle = gradle.replace(/versionName\s+"[^"]*"/, `versionName "${version}"`)
gradle = gradle.replace(/versionCode\s+\d+/, `versionCode ${versionCode}`)

if (!hadVersionName || !hadVersionCode) {
  console.warn(
    'set-android-version: build.gradle did not contain the expected versionName/versionCode ' +
      'fields; some replacements may not have applied. Please verify android/app/build.gradle.',
  )
}

writeFileSync(gradlePath, gradle)
console.log(`set-android-version: set versionName "${version}", versionCode ${versionCode} in ${gradlePath}`)
