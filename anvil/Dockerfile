FROM ubuntu:22.04

# Set environment variable for HOME directory
ENV HOME /root

# Install necessary packages
RUN apt-get update && apt-get install -y \
    curl \
    git \
    && rm -rf /var/lib/apt/lists/*

# Download and install Foundry toolchain
RUN curl -L https://foundry.paradigm.xyz | bash
RUN /root/.foundry/bin/foundryup

# Expose the default Anvil port
EXPOSE 8545

# Start Anvil with the correct path
ENTRYPOINT ["/root/.foundry/bin/anvil", "--host", "0.0.0.0"]

