#include "windivert.h"
#include <stdio.h>
#include <signal.h>
#include <winsock2.h>

#define MAX_PACKET_SIZE 9016

#define DIVERT_NO_LOCALNETS_DST "(" \
                   "(ip.DstAddr < 127.0.0.1 or ip.DstAddr > 127.255.255.255) and " \
                   "(ip.DstAddr < 10.0.0.0 or ip.DstAddr > 10.255.255.255) and " \
                   "(ip.DstAddr < 192.168.0.0 or ip.DstAddr > 192.168.255.255) and " \
                   "(ip.DstAddr < 172.16.0.0 or ip.DstAddr > 172.31.255.255) and " \
                   "(ip.DstAddr < 169.254.0.0 or ip.DstAddr > 169.254.255.255)" \
                   ")"

HANDLE handle;

static void sigintHandler(int sig __attribute__((unused))) {
    WinDivertClose(handle);
    exit(EXIT_SUCCESS);
}

int main(int argc, char *argv[]) {

    WINDIVERT_ADDRESS addr;
    char packet[MAX_PACKET_SIZE];
    PVOID packetData;
    UINT packetLen;
    UINT packetDataLen;
    PWINDIVERT_IPHDR ppIpHdr;

    char* filter = "outbound and ip and tcp and "
        "(tcp.DstPort == 443 or tcp.DstPort == 80) and "
        DIVERT_NO_LOCALNETS_DST;

    signal(SIGINT, sigintHandler);

    handle = WinDivertOpen(filter, WINDIVERT_LAYER_NETWORK, 0, 0);
    if (handle == INVALID_HANDLE_VALUE)
    {
        LPTSTR errormessage = NULL;
        FormatMessage(FORMAT_MESSAGE_ALLOCATE_BUFFER | FORMAT_MESSAGE_FROM_SYSTEM |
                      FORMAT_MESSAGE_IGNORE_INSERTS,
                      NULL, GetLastError(), MAKELANGID(LANG_ENGLISH, SUBLANG_DEFAULT),
                      (LPTSTR)&errormessage, 0, NULL);
        printf("%s", errormessage);
        exit(1);
    }

    printf("Capturing packets!");

    while (TRUE)
    {
        if (!WinDivertRecv(handle, packet, sizeof(packet), &addr, &packetLen))
        {
            printf("Receive error!");
        }

        if (WinDivertHelperParsePacket(packet, packetLen, &ppIpHdr,
                NULL, NULL, NULL, NULL, NULL, &packetData, &packetDataLen)) {

            UINT32 dst = ppIpHdr->DstAddr;
            struct in_addr dstAddr = (struct in_addr){ .s_addr = dst };
            struct sockaddr_in sin;
            sin.sin_family = AF_INET;
            sin.sin_addr = dstAddr;

            printf("Wants %s\n", inet_ntoa(dstAddr));

            /*
            int s = socket (AF_INET, SOCK_RAW, IPPROTO_RAW);
            int code = sendto (s, packet, packetLen, 0, (struct sockaddr*)&sin, sizeof (sin));
            if (code < 0) {
              perror("Failed to send");
            } else {
              puts("Sent IPV4");
            }
            */
        }

        // Modify packet.
        /*

        if (!WinDivertSend(handle, packet, packetLen, &addr, NULL))
        {
            // Handle send error
            continue;
        }
        */
    }

    return 0;

}
