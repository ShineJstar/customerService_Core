package controller

import (
	"fmt"
	"git.jsjit.cn/customerService/customerService_Core/common"
	"git.jsjit.cn/customerService/customerService_Core/logic"
	"git.jsjit.cn/customerService/customerService_Core/model"
	"git.jsjit.cn/customerService/customerService_Core/wechat"
	"git.jsjit.cn/customerService/customerService_Core/wechat/message"
	"github.com/gin-gonic/gin"
	"log"
)

type WeiXinController struct {
	wxContext *wechat.Wechat
	rooms     map[string]*logic.Room
}

func InitWeiXin(wxContext *wechat.Wechat, rooms map[string]*logic.Room) *WeiXinController {
	return &WeiXinController{wxContext: wxContext, rooms: rooms}
}

// 微信通信接口
func (c *WeiXinController) Listen(context *gin.Context) {
	wcServer := c.wxContext.GetServer(context.Request, context.Writer)

	//设置接收消息的处理方法
	wcServer.SetMessageHandler(func(msg message.MixMessage) (reply *message.Reply) {
		/*
			A 24小时新接入客户：
				1. 注册分配聊天房间
				2. 存储新客户、离线留言信息数据
			B 已在线的客户：
				1. 检索已分配的房间
				2. 存储聊天数据
		*/
		text := message.NewText(msg.Content)
		log.Printf("用户[%s]发来信息：%s \n", msg.FromUserName, text.Content)

		// 通信注册
		room, isNew := logic.InitRoom(msg.FromUserName)
		log.Printf("%#v", room)

		// 存储消息
		model.Message{
			CustomerToken: room.CustomerId,
			KfId:          room.KfId,
			KfAck:         false,
			Msg:           msg.Content,
			MsgType:       string(msg.MsgType),
			OperCode:      common.MessageFromCustomer,
		}.Insert()

		// 首次访问的客户
		if isNew {
			userInfo, err := c.wxContext.GetUser().GetUserInfo(msg.FromUserName)
			if err != nil {
				log.Printf("WeiXinController.wxContext.GetUser().GetUserInfo() is err：%v", err.Error())
			}

			// 客户数据持久化
			model.Customer{
				OpenId:       msg.FromUserName,
				NickName:     userInfo.Nickname,
				CustomerType: common.NormalCustomer,
				Sex:          userInfo.Sex,
				HeadImgUrl:   userInfo.Headimgurl,
				Address:      fmt.Sprintf("%s_%s", userInfo.Province, userInfo.City),
			}.InsertOrUpdate()

			if _, isOk := logic.GetOnlineKf(); !isOk {
				return &message.Reply{MsgType: message.MsgTypeText, MsgData: message.NewText(common.KF_REPLY)}
			}

			// 更新内存中的客户信息
			room.CustomerNickName = userInfo.Nickname
			room.CustomerHeadImgUrl = userInfo.Headimgurl
		}

		//logic.PrintRoomMap()

		return &message.Reply{message.MsgTypeText, nil}
	})

	//处理消息接收以及回复
	err := wcServer.Serve()
	if err != nil {
		fmt.Println(err)
		return
	}

	//发送接收成功
	wcServer.Send()
}
