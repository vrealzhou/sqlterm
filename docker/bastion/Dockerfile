FROM alpine

# Install OpenSSH and other necessary packages
RUN apk add --no-cache \
    openssh-server \
    openssh-client \
    bash \
    curl \
    ca-certificates \
    sqlite \
    mysql-client \
    postgresql-client \
    && ssh-keygen -A

# Create sqlterm user
RUN adduser -D -s /bin/bash sqlterm \
    && echo "sqlterm:sqlterm" | chpasswd

# Configure SSH
RUN mkdir -p /home/sqlterm/.ssh \
    && chown sqlterm:sqlterm /home/sqlterm/.ssh \
    && chmod 700 /home/sqlterm/.ssh

# Copy SSH configuration
COPY docker/bastion/sshd_config /etc/ssh/sshd_config

# Create directory for sqlterm binary
RUN mkdir -p /usr/local/bin

# Expose SSH port
EXPOSE 22

# Create startup script
RUN echo '#!/bin/bash' > /start.sh \
    && echo 'echo "Starting SSH daemon..."' >> /start.sh \
    && echo '/usr/sbin/sshd -D' >> /start.sh \
    && chmod +x /start.sh

CMD ["/start.sh"]
