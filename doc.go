// Copyright (c) 2018-2020. The asimov developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

/*
asimovd is a full-node asimov implementation written in Go.

The default options are sane for most users.  This means asimovd will work out
of the box' for most users. However, there are also a wide variety of flags that
can be used to control it.

The following section provides a usage overview which enumerates the flags.  An
interesting point to note is that the long form of all of these options
(except -C) can be specified in a configuration file that is automatically
parsed when asimovd starts up.  By default, the configuration file is located at
~/.asimovd/asimovd.conf on POSIX-style operating systems,
%LOCALAPPDATA%\Asimovd\asimovd.conf on Windows,
~/Library/Application Support/Asimovd/asimovd.conf on Mac OS, and
$home/asimovd/asimovd.conf on Plan9. The -C (--configfile) flag, as shown below,
can be used to override this location.

Usage:
  asimovd [OPTIONS]

Application Options:
  -V, --version             Display version information and exit
  -C, --configfile=         Path to configuration file
  -b, --datadir=            Directory to store data
      --logdir=             Directory to logger output.
      --statedir=           Directory to store state.
  -a, --addpeer=            Add a peer to connect with at startup
      --connect=            Connect only to the specified peers at startup
      --nolisten            Disable listening for incoming connections -- NOTE:
                            Listening is automatically disabled if the --connect
                            or --proxy options are used without also specifying
                            listen interfaces via --listen
      --listen=             Add an interface/port to listen for connections
                            (default all interfaces port: 8333, testnet: 18333)
      --maxpeers=           Max number of inbound and outbound peers (125)
      --nobanning           Disable banning of misbehaving peers
      --banduration=        How long to ban misbehaving peers.  Valid time units
                            are {s, m, h}.  Minimum 1 second (24h0m0s)
      --banthreshold=       Maximum allowed ban score before disconnecting and
                            banning misbehaving peers.
      --whitelist=          Add an IP network or IP that will not be banned.
                            (eg. 192.168.1.0/24 or ::1)
  -u, --rpcuser=            Username for RPC connections
  -P, --rpcpass=            Password for RPC connections
      --rpclimituser=       Username for limited RPC connections
      --rpclimitpass=       Password for limited RPC connections
      --rpccert=            File containing the certificate file
      --rpckey=             File containing the certificate key
      --rpcmaxclients=      Max number of RPC clients for standard connections
                            (10)
      --rpcmaxwebsockets=   Max number of RPC websocket connections (25)
      --norpc               Disable built-in RPC server -- NOTE: The RPC server
                            is disabled by default if no rpcuser/rpcpass or
                            rpclimituser/rpclimitpass is specified
      --notls               Disable TLS for the RPC server -- NOTE: This is only
                            allowed if the RPC server is bound to localhost
      --nodnsseed           Disable DNS seeding for peers
      --externalip=         Add an ip to the list of local addresses we claim to
                            listen on to peers
      --proxy=              Connect via SOCKS5 proxy (eg. 127.0.0.1:9050)
      --proxyuser=          Username for proxy server
      --proxypass=          Password for proxy server
      --onion=              Connect to tor hidden services via SOCKS5 proxy
                            (eg. 127.0.0.1:9050)
      --onionuser=          Username for onion proxy server
      --onionpass=          Password for onion proxy server
      --noonion             Disable connecting to tor hidden services
      --torisolation        Enable Tor stream isolation by randomizing user
                            credentials for each connection.
      --testnet             Use the test network
      --regtest             Use the regression test network
      --simnet              Use the simulation test network
      --addcheckpoint=      Add a custom checkpoint.  Format: '<height>:<hash>'
      --nocheckpoints       Disable built-in checkpoints.  Don't do this unless
                            you know what you're doing.
      --uacomment=          Comment to add to the user agent --
                            See BIP 14 for more information.
      --dbtype=             Database backend to use for the Block Chain (ffldb)
      --profile=            Enable HTTP profiling on given port -- NOTE port
                            must be between 1024 and 65536
      --cpuprofile=         Write CPU profile to the specified file
  -d, --debuglevel=         Logging level for all subsystems {trace, debug,
                            info, warn, error, critical} -- You may also specify
                            <subsystem>=<level>,<subsystem2>=<level>,... to set
                            the logger level for individual subsystems -- Use show
                            to list available subsystems (info)
      --upnp                Use UPnP to map our listening port outside of NAT
      --maxorphantx=        Max number of orphan transactions to keep in memory
                            (100)
      --nopeerbloomfilters  Disable bloom filtering support.
      --nocfilters          Disable committed filtering (CF) support.
      --blocksonly          Do not accept transactions from remote peers.


Help Options:
  -h, --help           Show this help message

*/
package main
