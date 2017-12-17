#include "windivert.h"
#include <stdio.h>
#include <signal.h>

#define MAX_PACKET_SIZE 9016

#define DIVERT_NO_LOCALNETS_DST "(" \
                   "(ip.DstAddr < 127.0.0.1 or ip.DstAddr > 127.255.255.255) and " \
                   "(ip.DstAddr < 10.0.0.0 or ip.DstAddr > 10.255.255.255) and " \
                   "(ip.DstAddr < 192.168.0.0 or ip.DstAddr > 192.168.255.255) and " \
                   "(ip.DstAddr < 172.16.0.0 or ip.DstAddr > 172.31.255.255) and " \
                   "(ip.DstAddr < 169.254.0.0 or ip.DstAddr > 169.254.255.255)" \
                   ")"

HANDLE handle;

static void sigint_handler(int sig __attribute__((unused))) {
    WinDivertClose(handle);
    exit(EXIT_SUCCESS);
}

int main(int argc, char *argv[]) {

    WINDIVERT_ADDRESS addr;
    char packet[MAX_PACKET_SIZE];
    UINT packetLen;

    char* filter = "inbound and ip and tcp and "
        "(tcp.DstPort == 443 or tcp.DstPort == 80) and "
        DIVERT_NO_LOCALNETS_DST;

    signal(SIGINT, sigint_handler);

    handle = WinDivertOpen(filter,  WINDIVERT_LAYER_NETWORK, -1000, 0);
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
            continue;
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
