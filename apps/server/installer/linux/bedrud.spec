# Define systemd unit dir if the macro is not provided by the build environment
# (e.g. Ubuntu's rpm package lacks systemd-rpm-macros)
%{!?_unitdir: %define _unitdir /usr/lib/systemd/system}

# Pre-built static binary (often cross-compiled). Host strip/debuginfo cannot
# process foreign-arch ELF (e.g. aarch64 binary on x86_64 CI runners).
%global __strip /bin/true
%global debug_package %{nil}

# Ubuntu's rpm macros map _sharedstatedir to /usr/com; we want /var/lib.
%define _sharedstatedir /var/lib

Name:           bedrud
Version:        VERSION_PLACEHOLDER
Release:        1%{?dist}
Summary:        Self-hosted video meeting server
License:        AGPL-3.0-or-later
URL:            https://github.com/themadorg/bedrud
Requires:       glibc

%description
Bedrud is a self-hosted video meeting server that bundles a web UI,
REST API, and LiveKit WebRTC media server in a single binary.
Supports Let's Encrypt TLS and SQLite/PostgreSQL databases.

%install
install -Dm755 bedrud %{buildroot}%{_bindir}/bedrud
install -Dm644 bedrud.service %{buildroot}%{_unitdir}/bedrud.service
install -Dm644 livekit.service %{buildroot}%{_unitdir}/livekit.service
install -dm755 %{buildroot}%{_sysconfdir}/bedrud
install -dm755 %{buildroot}%{_sharedstatedir}/bedrud
install -dm755 %{buildroot}/var/log/bedrud
install -dm755 %{buildroot}%{_docdir}/bedrud/examples
if [ -d examples ]; then
    install -m644 examples/* %{buildroot}%{_docdir}/bedrud/examples/
fi
if [ -f bedrud.1 ]; then
    install -Dm644 bedrud.1 %{buildroot}%{_mandir}/man1/bedrud.1
fi

%files
%{_bindir}/bedrud
%{_unitdir}/bedrud.service
%{_unitdir}/livekit.service
%dir %{_sysconfdir}/bedrud
%dir %{_sharedstatedir}/bedrud
%dir /var/log/bedrud
%dir %{_docdir}/bedrud
%dir %{_docdir}/bedrud/examples
%{_docdir}/bedrud/examples/*
%{_mandir}/man1/bedrud.1*

%post
getent group bedrud >/dev/null || groupadd -r bedrud
getent passwd bedrud >/dev/null || \
    useradd -r -g bedrud -s /usr/sbin/nologin -d %{_sharedstatedir}/bedrud bedrud
chown -R bedrud:bedrud %{_sharedstatedir}/bedrud /var/log/bedrud
systemctl daemon-reload >/dev/null 2>&1 || :
if [ -f /etc/bedrud/config.yaml ] && [ -f /etc/bedrud/livekit.yaml ]; then
    systemctl enable livekit.service bedrud.service >/dev/null 2>&1 || :
    systemctl restart livekit.service bedrud.service >/dev/null 2>&1 || :
else
    systemctl enable livekit.service bedrud.service >/dev/null 2>&1 || :
    echo ""
    echo "Bedrud installed. Generate config + LiveKit setup:"
    echo "  sudo bedrud install"
    echo "Example configs: /usr/share/doc/bedrud/examples/"
    echo "Docs: https://themadorg.github.io/bedrud/"
fi

%preun
if [ $1 -eq 0 ]; then
    systemctl stop bedrud.service livekit.service >/dev/null 2>&1 || :
    systemctl disable bedrud.service livekit.service >/dev/null 2>&1 || :
fi

%postun
if [ $1 -eq 0 ]; then
    systemctl daemon-reload >/dev/null 2>&1 || :
fi
