package matchmake_extension

import (
	"math"

	"github.com/PretendoNetwork/mario-kart-7-secure/globals"
	nex "github.com/PretendoNetwork/nex-go"
	match_making "github.com/PretendoNetwork/nex-protocols-go/match-making"
	matchmake_extension "github.com/PretendoNetwork/nex-protocols-go/matchmake-extension"
	"github.com/PretendoNetwork/nex-protocols-go/notifications"
	common_globals "github.com/PretendoNetwork/nex-protocols-common-go/globals"
	"encoding/json"
)

func AutoMatchmake_Postpone(err error, client *nex.Client, callID uint32, matchmakeSession *match_making.MatchmakeSession, message string) {
	missingHandler := false
	if commonMatchmakeExtensionProtocol.CleanupSearchMatchmakeSessionHandler == nil {
		logger.Warning("MatchmakeExtension::AutoMatchmake_Postpone missing CleanupSearchMatchmakeSessionHandler!")
		missingHandler = true
	}
	if missingHandler {
		return
	}
	//This is the best way I found to copy the full object. And I still hate it.
	tmp, _ := json.Marshal(matchmakeSession)
	matchmakeSessionCopy := match_making.NewMatchmakeSession()
	json.Unmarshal(tmp, &matchmakeSessionCopy)
	searchMatchmakeSession := commonMatchmakeExtensionProtocol.CleanupSearchMatchmakeSessionHandler(*matchmakeSessionCopy)
	sessionIndex := FindSearchMatchmakeSession(searchMatchmakeSession)
	if sessionIndex == math.MaxUint32 {
		session := common_globals.CommonMatchmakeSession{
			SearchMatchmakeSession: searchMatchmakeSession,
			GameMatchmakeSession:   *matchmakeSession,
		}
		sessionIndex = len(common_globals.Sessions)
		common_globals.Sessions = append(common_globals.Sessions, session)
		common_globals.Sessions[sessionIndex].GameMatchmakeSession.Gathering.ID = uint32(sessionIndex)
		common_globals.Sessions[sessionIndex].GameMatchmakeSession.Gathering.OwnerPID = client.PID()
		common_globals.Sessions[sessionIndex].GameMatchmakeSession.Gathering.HostPID = client.PID()
	}

	common_globals.Sessions[sessionIndex].PlayersByConnectionId = append(common_globals.Sessions[sessionIndex].PlayersByConnectionId, client.ConnectionID())

	rmcResponseStream := nex.NewStreamOut(globals.NEXServer)
	rmcResponseStream.WriteString("MatchmakeSession")
	lengthStream := nex.NewStreamOut(globals.NEXServer)
	lengthStream.WriteStructure(&common_globals.Sessions[sessionIndex].GameMatchmakeSession)
	matchmakeSessionLength := uint32(len(lengthStream.Bytes()))
	rmcResponseStream.WriteUInt32LE(matchmakeSessionLength + 4)
	rmcResponseStream.WriteUInt32LE(matchmakeSessionLength)
	rmcResponseStream.WriteStructure(&common_globals.Sessions[sessionIndex].GameMatchmakeSession)

	rmcResponseBody := rmcResponseStream.Bytes()

	rmcResponse := nex.NewRMCResponse(matchmake_extension.ProtocolID, callID)
	rmcResponse.SetSuccess(matchmake_extension.MethodAutoMatchmake_Postpone, rmcResponseBody)

	rmcResponseBytes := rmcResponse.Bytes()

	responsePacket, _ := nex.NewPacketV0(client, nil)

	responsePacket.SetVersion(0)
	responsePacket.SetSource(0xA1)
	responsePacket.SetDestination(0xAF)
	responsePacket.SetType(nex.DataPacket)
	responsePacket.SetPayload(rmcResponseBytes)

	responsePacket.AddFlag(nex.FlagNeedsAck)
	responsePacket.AddFlag(nex.FlagReliable)

	globals.NEXServer.Send(responsePacket)

	rmcMessage := nex.NewRMCRequest()
	rmcMessage.SetProtocolID(notifications.ProtocolID)
	rmcMessage.SetCallID(0xffff0000 + callID)
	rmcMessage.SetMethodID(notifications.MethodProcessNotificationEvent)

	oEvent := notifications.NewNotificationEvent()
	oEvent.PIDSource = common_globals.Sessions[sessionIndex].GameMatchmakeSession.Gathering.HostPID
	oEvent.Type = 3001 // New participant
	oEvent.Param1 = uint32(sessionIndex)
	oEvent.Param2 = client.PID()

	stream := nex.NewStreamOut(globals.NEXServer)
	oEventBytes := oEvent.Bytes(stream)
	rmcMessage.SetParameters(oEventBytes)
	rmcMessageBytes := rmcMessage.Bytes()

	targetClient := globals.NEXServer.FindClientFromPID(uint32(common_globals.Sessions[sessionIndex].GameMatchmakeSession.Gathering.HostPID))

	messagePacket, _ := nex.NewPacketV0(targetClient, nil)
	messagePacket.SetVersion(1)
	messagePacket.SetSource(0xA1)
	messagePacket.SetDestination(0xAF)
	messagePacket.SetType(nex.DataPacket)
	messagePacket.SetPayload(rmcMessageBytes)

	messagePacket.AddFlag(nex.FlagNeedsAck)
	messagePacket.AddFlag(nex.FlagReliable)

	globals.NEXServer.Send(messagePacket)
}
