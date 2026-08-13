#!/bin/bash

set -e

# Install Earthly (for release branches)
echo "Installing Earthly..."
curl -fsSLo /usr/local/bin/earthly https://github.com/earthly/earthly/releases/latest/download/earthly-linux-amd64
chmod +x /usr/local/bin/earthly
/usr/local/bin/earthly bootstrap

# Install Nix (for main branch)
echo "Installing Nix..."
apt-get update && apt-get install -y nix-bin

# Configure Nix
mkdir -p /etc/nix

# The Renovate container can't run Nix as sandboxed, so HOME=/homeless-shelter. Any build that
# writes to $HOME, will cause subsequent builds to fail if that directory isn't cleaned up first.
cat > /usr/local/bin/nix-clean-homeless-shelter << 'EOF'
#!/bin/sh
rm -rf /homeless-shelter
EOF
chmod +x /usr/local/bin/nix-clean-homeless-shelter

cat > /etc/nix/nix.conf << 'EOF'
# Enable flakes and the nix command (e.g. nix run, nix build).
experimental-features = nix-command flakes

# Run builds as the calling user, not dedicated nixbld users. This avoids
# needing to create the nixbld group and users in this ephemeral container.
build-users-group =

# One build at a time, so no build starts before we can clean up /homeless-shelter.
max-jobs = 1

# Removes /homeless-shelter after each successful build. The crossplane-nix launcher covers any
# failed builds this misses.
post-build-hook = /usr/local/bin/nix-clean-homeless-shelter

# Use the Crossplane Cachix cache to download pre-built binaries from CI.
extra-substituters = https://crossplane.cachix.org
extra-trusted-public-keys = crossplane.cachix.org-1:NJluVUN9TX0rY/zAxHYaT19Y5ik4ELH4uFuxje+62d4=
EOF

# Renovate installs its own Nix when it updates flake.lock. It goes earlier on
# PATH than ours and ignores the config above, so all nix commands we run (e.g.
# postUpgradeTasks) will go through this launcher, which pins both the binary
# and the config it reads.
cat > /usr/local/bin/crossplane-nix << 'EOF'
#!/bin/bash
# ensure any leftover /homeless-shelter from a failed build is cleaned up before we start
/usr/local/bin/nix-clean-homeless-shelter
exec env NIX_CONF_DIR=/etc/nix /usr/bin/nix "$@"
EOF
chmod +x /usr/local/bin/crossplane-nix

echo "Nix $(crossplane-nix --version) installed successfully"

renovate
