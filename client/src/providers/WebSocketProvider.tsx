import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
} from "react";
import { useConversationStore } from "../store/conversationstore";
import wsService, { WSEventType } from "../service/ws";
import toast from "react-hot-toast";
import { MessageType } from "../lib/types";
import { useNavigate } from "react-router-dom";

interface WebSocketContextType {
  isConnected: boolean;
  isThinking: boolean;
  currentThinking: string;
  finalThinking: string | null;
  sendMessage: (message: string) => Promise<void>;
  startConversation: (model: string, message: string) => Promise<void>;
  resumeConversation: (conversationId: string) => Promise<void>;
  clearThinkingState: () => void;
}

const WebSocketContext = createContext<WebSocketContextType>({
  isConnected: false,
  isThinking: false,
  currentThinking: "",
  finalThinking: null,
  sendMessage: async () => {},
  startConversation: async () => {},
  resumeConversation: async () => {},
  clearThinkingState: () => {},
});

export const useWebSocket = () => useContext(WebSocketContext);

interface WebSocketProviderProps {
  children: React.ReactNode;
}

const WebSocketProvider = ({ children }: WebSocketProviderProps) => {
  const navigate = useNavigate();
  const [isConnected, setIsConnected] = useState(false);
  const [isThinking, setIsThinking] = useState(false);
  const [thinkingStartTime, setThinkingStartTime] = useState<number | null>(
    null
  );
  const [thinkingEndTime, setThinkingEndTime] = useState<number | null>(null);
  const [finalThinking, setFinalThinking] = useState<string | null>(null);
  const [currentThinking, setCurrentThinking] = useState("");
  const [currentResponse, setCurrentResponse] = useState("");
  const currentUserMessageRef = useRef<string>("");
  const activeMessageIdRef = useRef<string | null>(null);

  const {
    fetchInitialData,
    error,
    selectedConversation,
    addMessageToConversation,
    createNewConversation,
    updateConversationTitle,
    updateMessageContent,
    updateMessageWithThinking,
    selectedModel,
  } = useConversationStore();

  useEffect(() => {
    const initializeApp = async () => {
      await fetchInitialData();
      const connected = await wsService.connect();
      setIsConnected(connected);
    };

    initializeApp();

    // Cleanup on unmount
    return () => {
      wsService.disconnect();
    };
  }, [fetchInitialData]);

  useEffect(() => {
    const eventHandlers: Record<WSEventType, (data: any) => void> = {
      connected: () => {
        setIsConnected(true);
        toast.success("Conectado al servidor");
      },
      disconnected: () => {
        setIsConnected(false);
        toast.error("Desconectado del servidor");
      },
      thinking_start: () => {
        setIsThinking(true);
        setCurrentThinking("");
        setThinkingStartTime(Date.now());
        setFinalThinking(null);
      },
      thinking_chunk: (content) => {
        if (content) {
          setCurrentThinking((prev) => prev + content);
        }
      },
      thinking_end: () => {
        setIsThinking(false);
        setThinkingEndTime(Date.now());
        if (selectedConversation && currentThinking) {
          const messageId = crypto.randomUUID();
          activeMessageIdRef.current = messageId;

          const thinkingTimeInSeconds = thinkingStartTime
            ? (Date.now() - thinkingStartTime) / 1000
            : null;

          const initialMessage: MessageType = {
            ID: messageId,
            ConversationID: selectedConversation.ID,
            Role: "assistant",
            Content: "", // Empty content initially - will be filled by response chunks
            RawContent: "",
            Thinking: currentThinking,
            ThinkingTime: thinkingTimeInSeconds,
            CreatedAt: new Date().toISOString(),
          };

          addMessageToConversation(selectedConversation.ID, initialMessage);
        }
      },
      conversation_started: (conversationId) => {
        if (conversationId) {
          toast.success("Nueva conversación iniciada");

          const userMessage: MessageType = {
            ID: crypto.randomUUID(),
            ConversationID: conversationId,
            Role: "user",
            Content: currentUserMessageRef.current,
            RawContent: currentUserMessageRef.current,
            Thinking: null,
            ThinkingTime: null,
            CreatedAt: new Date().toISOString(),
          };

          createNewConversation(
            conversationId,
            selectedModel?.model || "Modelo desconocido",
            userMessage
          );

          currentUserMessageRef.current = "";

          navigate(`/chat/${conversationId}`);
        }
      },
      conversation_resumed: (conversationId) => {
        if (conversationId) {
          console.log(`Conversación reanudada: ${conversationId}`);
          toast.success(`Conversación reanudada`);
        } else {
          console.warn(
            "Se recibió evento conversation_resumed sin ID de conversación"
          );
        }
      },
      response_chunk: (content) => {
        setCurrentResponse((prev) => prev + content);

        if (selectedConversation) {
          // If this is the first chunk, create a new message
          if (!activeMessageIdRef.current) {
            const messageId = crypto.randomUUID();
            activeMessageIdRef.current = messageId;

            const assistantMessage: MessageType = {
              ID: messageId,
              ConversationID: selectedConversation.ID,
              Role: "assistant",
              Content: content,
              RawContent: content,
              Thinking: currentThinking,
              ThinkingTime: thinkingStartTime
                ? (Date.now() - thinkingStartTime) / 1000
                : null,
              CreatedAt: new Date().toISOString(),
            };

            addMessageToConversation(selectedConversation.ID, assistantMessage);
          } else {
            // If we already have an active message, update it
            const updatedContent = currentResponse + content;
            updateMessageContent(
              selectedConversation.ID,
              activeMessageIdRef.current,
              updatedContent
            );
          }
        }
      },

      response_done: () => {
        if (currentResponse && selectedConversation) {
          const thinkingTimeInSeconds =
            thinkingStartTime && thinkingEndTime
              ? (thinkingEndTime - thinkingStartTime) / 1000
              : null;

          console.log(`Thinking time: ${thinkingTimeInSeconds}s`);

          // If we already created a streaming message, just update it with final content
          if (activeMessageIdRef.current) {
            // Update the message with final content and thinking
            updateMessageWithThinking(
              selectedConversation.ID,
              activeMessageIdRef.current,
              currentResponse,
              finalThinking || currentThinking || null,
              thinkingTimeInSeconds
            );

            // Reset active message ID
            activeMessageIdRef.current = null;
          } else {
            // Create new message if we didn't stream (fallback)
            const assistantMessage: MessageType = {
              ID: crypto.randomUUID(),
              ConversationID: selectedConversation.ID,
              Role: "assistant",
              Content: currentResponse,
              RawContent: currentResponse,
              Thinking: finalThinking || currentThinking || null,
              ThinkingTime: thinkingTimeInSeconds,
              CreatedAt: new Date().toISOString(),
            };

            addMessageToConversation(selectedConversation.ID, assistantMessage);
          }

          // Check if this is the first assistant message to update title
          // const isFirstAssistantMessage =
          //   selectedConversation.Messages?.filter(m => m.Role === "assistant").length <= 1;

          // if (isFirstAssistantMessage) {
          //   let title = currentResponse.slice(0, 30);
          //   if (currentResponse.length > 30) title += "...";
          //   updateConversationTitle(selectedConversation.ID, title);
          // }
        }

        // Reset state
        setCurrentResponse("");
        activeMessageIdRef.current = null;
        setThinkingStartTime(null);
        setThinkingEndTime(null);
      },
      error: (errorMsg) => {
        console.error(`Error: ${errorMsg}`);
      },
    };

    Object.entries(eventHandlers).forEach(([event, handler]) => {
      wsService.addEventListener(event as WSEventType, handler);
    });

    return () => {
      Object.entries(eventHandlers).forEach(([event, handler]) => {
        wsService.removeEventListener(event as WSEventType, handler);
      });
    };
  }, [
    selectedConversation,
    currentThinking,
    currentResponse,
    selectedModel,
    addMessageToConversation,
    createNewConversation,
    updateConversationTitle,
    isThinking,
    navigate,
  ]);

  const sendMessage = useCallback(
    async (message: string) => {
      if (!selectedConversation) {
  toast.error("No hay una conversación activa");
        return;
      }

      const userMessage: MessageType = {
        ID: crypto.randomUUID(),
        ConversationID: selectedConversation.ID,
        Role: "user",
        Content: message,
        RawContent: message,
        Thinking: null,
        ThinkingTime: null,
        CreatedAt: new Date().toISOString(),
      };

      addMessageToConversation(selectedConversation.ID, userMessage);

      const success = await wsService.sendMessage(
        selectedConversation.ID,
        message,
        selectedConversation.Model
      );

      if (!success) {
  toast.error("Error al enviar el mensaje");
        return;
      }
    },
    [selectedConversation, addMessageToConversation]
  );

  const startConversation = useCallback(
    async (model: string, message: string) => {
      if (!model) {
    toast.error("No hay ningún modelo seleccionado");
        return;
      }

      currentUserMessageRef.current = message;

      const success = await wsService.startConversation(model, message);

      if (!success) {
  toast.error("Error al iniciar la conversación");
      }
    },
    []
  );

  const resumeConversation = useCallback(async (conversationId: string) => {
    console.log(`Resuming conversation: ${conversationId}`);
    const success = await wsService.resumeConversation(conversationId);

    if (!success) {
  toast.error("Error al reanudar la conversación");
    }
  }, []);

  const clearThinkingState = useCallback(() => {
    setCurrentThinking("");
    setFinalThinking(null);
    setThinkingStartTime(null);
    setThinkingEndTime(null);
  }, []);

  const contextValue: WebSocketContextType = {
    isConnected,
    isThinking,
    currentThinking,
    finalThinking,
    sendMessage,
    startConversation,
    resumeConversation,
    clearThinkingState,
  };

  if (error) {
    return (
      <div className="h-screen flex items-center justify-center bg-gray-950 text-gray-200">
        <div className="text-center">
          <h2 className="text-xl font-semibold mb-2">Error al cargar</h2>
          <p className="text-gray-400">{error}</p>
        </div>
      </div>
    );
  }

  return (
    <WebSocketContext.Provider value={contextValue}>
      <div className="min-h-screen bg-gray-950">{children}</div>
    </WebSocketContext.Provider>
  );
};

export default WebSocketProvider;
